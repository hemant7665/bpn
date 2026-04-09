package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"

	"project-serverless/apps/subgraph-import/orchestrator"
	"project-serverless/internal/logger"
	"project-serverless/internal/middleware"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"
)

var gqlServer *handler.Server
var corsHandler http.Handler

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	req, err := http.NewRequest(request.RequestContext.HTTP.Method, request.RawPath, bytes.NewBufferString(request.Body))
	if err != nil {
		return events.LambdaFunctionURLResponse{StatusCode: 500, Body: "Internal server error"}, nil
	}
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}
	req = req.WithContext(ctx)
	responseWriter := &responseRecorder{headers: make(http.Header), statusCode: 200}
	corsHandler.ServeHTTP(responseWriter, req)
	headers := make(map[string]string)
	for key, values := range responseWriter.headers {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return events.LambdaFunctionURLResponse{
		StatusCode: responseWriter.statusCode,
		Headers:    headers,
		Body:       responseWriter.body.String(),
	}, nil
}

type responseRecorder struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Header() http.Header { return r.headers }
func (r *responseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *responseRecorder) WriteHeader(statusCode int) { r.statusCode = statusCode }

func main() {
	_ = godotenv.Load(".env")
	if os.Getenv("AWS_ENDPOINT_URL_OVERRIDE") != "" {
		os.Setenv("AWS_ENDPOINT_URL", os.Getenv("AWS_ENDPOINT_URL_OVERRIDE"))
	}

	svc, err := orchestrator.NewService()
	if err != nil {
		panic("Failed to initialize import orchestrator: " + err.Error())
	}

	gqlServer = handler.NewDefaultServer(NewExecutableSchema(Config{
		Resolvers: &Resolver{Orchestrator: svc},
	}))

	// All import operations require JWT (login via subgraph-user for token).
	authWrapped := middleware.AuthMiddleware([]string{})(gqlServer)
	corsHandler = authWrapped

	if os.Getenv("ENVIRONMENT") == "local" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "4004"
		}
		addr := ":" + port
		logger.Info("starting_import_graphql_playground", map[string]any{"url": fmt.Sprintf("http://localhost%s/", addr)})
		http.Handle("/", playground.Handler("Import GraphQL playground", "/query"))
		http.Handle("/query", corsHandler)
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(err)
		}
	} else {
		lambda.Start(handleRequest)
	}
}
