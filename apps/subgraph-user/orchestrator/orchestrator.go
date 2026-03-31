package orchestrator

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/domain"
	"project-serverless/internal/logger"
	"project-serverless/internal/validator"
)

type Service interface {
	GetUser(ctx context.Context, id string) (*domain.UserSummary, error)
	ListUsers(ctx context.Context) ([]domain.UserSummary, error)
	CreateUser(ctx context.Context, name string, email string) (*domain.User, error)
	DeleteUser(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, id string, name string, email string) (*domain.User, error)
}

type serviceImpl struct {
	lambdaClient LambdaInvoker
}

func NewService() (Service, error) {
	var cfg aws.Config
	var err error

	ctx := context.TODO()
	environment := os.Getenv("ENVIRONMENT")

	if environment == "local" {
		logger.Info("initializing_orchestrator", map[string]any{"environment": "local"})
		awsEndpoint := os.Getenv("AWS_LAMBDA_ENDPOINT")
		if awsEndpoint == "" {
			awsEndpoint = os.Getenv("AWS_ENDPOINT_URL")
		}

		if awsEndpoint == "" {
			return nil, apperrors.NewValidation("AWS_ENDPOINT_URL or AWS_LAMBDA_ENDPOINT must be set in local environment")
		}

		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: awsEndpoint, SigningRegion: "us-east-1"}, nil
				},
			)),
		)
	} else {
		logger.Info("initializing_orchestrator", map[string]any{"environment": environment})
		cfg, err = config.LoadDefaultConfig(ctx)
	}

	if err != nil {
		logger.Error("failed_to_load_aws_config", map[string]any{"environment": environment, "error": err.Error()})
		return nil, apperrors.NewInternal("service unavailable", err)
	}

	logger.Info("orchestrator_initialized", map[string]any{"environment": environment})
	return NewServiceWithClient(lambda.NewFromConfig(cfg)), nil
}

func NewServiceWithClient(client LambdaInvoker) Service {
	return &serviceImpl{
		lambdaClient: client,
	}
}

func (s *serviceImpl) invokeLambda(ctx context.Context, functionName string, payload interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.NewInternal("failed to marshal lambda payload", err)
	}

	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payloadBytes,
	}

	result, err := s.lambdaClient.Invoke(ctx, input)
	if err != nil {
		return nil, apperrors.NewInternal("failed to invoke lambda", err)
	}

	if result.FunctionError != nil {
		return nil, apperrors.NewInternal("lambda function error: "+*result.FunctionError, nil)
	}

	return result.Payload, nil
}

func (s *serviceImpl) GetUser(ctx context.Context, id string) (*domain.UserSummary, error) {
	idInt, err := validator.ParsePositiveIntID(id)
	if err != nil {
		return nil, err
	}

	lambdaName := getLambdaName("LAMBDA_GET_USER_NAME", "getUser")

	payload := map[string]interface{}{
		"id": idInt,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("get_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.UserSummary
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		logger.Error("get_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, apperrors.NewInternal("invalid getUser lambda response", err)
	}

	return &user, nil

}

func (s *serviceImpl) ListUsers(ctx context.Context) ([]domain.UserSummary, error) {
	lambdaName := getLambdaName("LAMBDA_LIST_USERS_NAME", "listUsers")

	responseBody, err := s.invokeLambda(ctx, lambdaName, map[string]interface{}{})
	if err != nil {
		logger.Error("list_users_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var users []domain.UserSummary
	err = json.Unmarshal(responseBody, &users)
	if err != nil {
		logger.Error("list_users_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, apperrors.NewInternal("invalid listUsers lambda response", err)
	}

	return users, nil
}

func (s *serviceImpl) CreateUser(ctx context.Context, name string, email string) (*domain.User, error) {

	lambdaName := getLambdaName("LAMBDA_CREATE_USER_NAME", "createUser")

	payload := map[string]interface{}{
		"name":  name,
		"email": email,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("create_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		logger.Error("create_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, apperrors.NewInternal("invalid createUser lambda response", err)
	}

	return &user, nil
}

func (s *serviceImpl) DeleteUser(ctx context.Context, id string) (*domain.User, error) {
	idInt, err := validator.ParsePositiveIntID(id)
	if err != nil {
		return nil, err
	}

	lambdaName := getLambdaName("LAMBDA_DELETE_USER_NAME", "deleteUser")

	payload := map[string]interface{}{
		"id": idInt,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("delete_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		logger.Error("delete_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, apperrors.NewInternal("invalid deleteUser lambda response", err)
	}

	return &user, nil

}

func (s *serviceImpl) UpdateUser(ctx context.Context, id string, name string, email string) (*domain.User, error) {
	idInt, err := validator.ParsePositiveIntID(id)
	if err != nil {
		return nil, err
	}

	lambdaName := getLambdaName("LAMBDA_UPDATE_USER_NAME", "updateUser")

	payload := map[string]interface{}{
		"id":    idInt,
		"name":  name,
		"email": email,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("update_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		logger.Error("update_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, apperrors.NewInternal("invalid updateUser lambda response", err)
	}

	return &user, nil

}

func getLambdaName(envVar string, fallback string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return fallback
}
