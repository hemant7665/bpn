package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"project-serverless/internal/auth"
	"project-serverless/internal/logger"
)

// AuthMiddleware verifies JWT at the subgraph boundary (BluePrint-style), then attaches
// Authorization + claims to request context for the orchestrator to forward to Lambdas.
func AuthMiddleware(skipOperationNames []string) func(http.Handler) http.Handler {
	skip := make(map[string]struct{}, len(skipOperationNames))
	for _, op := range skipOperationNames {
		skip[strings.ToLower(strings.TrimSpace(op))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			shouldSkip, proceed := graphqlShouldSkipAuth(r, skip)
			if !proceed {
				writeGraphQLAuthError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			if shouldSkip {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			authHeader := r.Header.Get("Authorization")
			if strings.TrimSpace(authHeader) == "" {
				logger.Info("graphql_auth_missing_authorization", nil)
				writeGraphQLAuthError(w, http.StatusUnauthorized, "missing authorization")
				return
			}

			rawToken, err := auth.ExtractBearerToken(authHeader)
			if err != nil {
				writeGraphQLAuthError(w, http.StatusUnauthorized, "invalid authorization header")
				return
			}

			claims, err := auth.ValidateToken(rawToken)
			if err != nil {
				logger.Info("graphql_auth_invalid_token", map[string]any{"error": err.Error()})
				writeGraphQLAuthError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx = auth.WithAuthorization(ctx, authHeader)
			ctx = auth.WithClaims(ctx, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func graphqlShouldSkipAuth(r *http.Request, skip map[string]struct{}) (skipAuth bool, ok bool) {
	if r.Method != http.MethodPost || !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return true, true
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return false, false
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var gqlRequest struct {
		Query         string `json:"query"`
		OperationName string `json:"operationName"`
	}
	if err := json.Unmarshal(bodyBytes, &gqlRequest); err != nil {
		return true, true
	}

	if gqlRequest.OperationName != "" {
		opLower := strings.ToLower(gqlRequest.OperationName)
		if _, s := skip[opLower]; s {
			return true, true
		}
		if opLower == "introspectionquery" {
			return true, true
		}
	}

	q := gqlRequest.Query
	if strings.Contains(q, "__schema") || strings.Contains(q, "__Type") {
		return true, true
	}

	name := gqlRequest.OperationName
	if name == "" && q != "" {
		name = extractBareOperationName(q)
	}
	// Anonymous operations: `mutation { login(...) }` has no operation name; match first root field.
	if name == "" && q != "" {
		name = extractAnonymousMutationFirstField(q)
	}
	if name != "" {
		if _, s := skip[strings.ToLower(name)]; s {
			return true, true
		}
	}

	return false, true
}

var operationNameRegex = regexp.MustCompile(`(?i)(query|mutation|subscription)\s+([A-Za-z_][A-Za-z0-9_]*)`)

// anonymousMutationFirstField matches `mutation { login` / `mutation{createUser` (no operation name).
var anonymousMutationFirstFieldRegex = regexp.MustCompile(`(?i)mutation\s*\{\s*([A-Za-z_][A-Za-z0-9_]*)`)

func extractBareOperationName(query string) string {
	m := operationNameRegex.FindStringSubmatch(query)
	if len(m) >= 3 {
		return m[2]
	}
	return ""
}

func extractAnonymousMutationFirstField(query string) string {
	m := anonymousMutationFirstFieldRegex.FindStringSubmatch(query)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func writeGraphQLAuthError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]string{
			{"message": message},
		},
	})
}
