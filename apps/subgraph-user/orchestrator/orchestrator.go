package orchestrator

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/validator"
)

// Service is the subgraph facade over command/query Lambdas.
type Service interface {
	GetUser(ctx context.Context, id string) (*domain.UserSummary, error)
	ListUsers(ctx context.Context, skip, limit int, username, email *string) (*domain.UserListPayload, error)
	CreateUser(ctx context.Context, input CreateUserParams) (*domain.User, error)
	Login(ctx context.Context, email string, password string) (string, error)
	DeleteUser(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, id string, input UpdateUserParams) (*domain.User, error)
}

// CreateUserParams maps GraphQL create input to Lambda JSON.
type CreateUserParams struct {
	TenantID    string
	Username    string
	Email       string
	Password    string
	PhoneNo     string
	DateOfBirth string
	Gender      string
}

// UpdateUserParams maps GraphQL update input to Lambda JSON.
type UpdateUserParams struct {
	Username    string
	Email       string
	PhoneNo     string
	DateOfBirth string
	Gender      string
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
			return nil, svcerrors.Validation("AWS_ENDPOINT_URL or AWS_LAMBDA_ENDPOINT must be set in local environment")
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
		return nil, svcerrors.Internal("service unavailable", err)
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
		return nil, svcerrors.Internal("failed to marshal lambda payload", err)
	}

	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payloadBytes,
	}

	result, err := s.lambdaClient.Invoke(ctx, input)
	if err != nil {
		return nil, svcerrors.Internal("failed to invoke lambda", err)
	}

	if result.FunctionError != nil {
		detail := *result.FunctionError
		if len(result.Payload) > 0 {
			detail = detail + ": " + string(result.Payload)
		}
		return nil, svcerrors.Internal("lambda function error: "+detail, nil)
	}

	return result.Payload, nil
}

func mergeAuthorization(ctx context.Context, payload map[string]interface{}) {
	if hdr := auth.AuthorizationFromContext(ctx); hdr != "" {
		payload["authorization"] = hdr
	}
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
	mergeAuthorization(ctx, payload)

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("get_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.UserSummary
	if err = json.Unmarshal(responseBody, &user); err != nil {
		logger.Error("get_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, svcerrors.Internal("invalid getUser lambda response", err)
	}

	return &user, nil
}

func (s *serviceImpl) ListUsers(ctx context.Context, skip int, limit int, username, email *string) (*domain.UserListPayload, error) {
	lambdaName := getLambdaName("LAMBDA_LIST_USERS_NAME", "listUsers")

	payload := map[string]interface{}{
		"skip": skip,
	}
	if limit > 0 {
		payload["limit"] = limit
	}
	if username != nil {
		payload["username"] = *username
	}
	if email != nil {
		payload["email"] = *email
	}
	mergeAuthorization(ctx, payload)

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("list_users_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var out struct {
		Items []domain.UserSummary `json:"items"`
		Total int64                `json:"total"`
	}
	if err = json.Unmarshal(responseBody, &out); err != nil {
		logger.Error("list_users_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, svcerrors.Internal("invalid listUsers lambda response", err)
	}
	return &domain.UserListPayload{Items: out.Items, Total: out.Total}, nil
}

func (s *serviceImpl) CreateUser(ctx context.Context, input CreateUserParams) (*domain.User, error) {
	lambdaName := getLambdaName("LAMBDA_CREATE_USER_NAME", "createUser")

	payload := map[string]interface{}{
		"tenant_id":    input.TenantID,
		"username":     input.Username,
		"email":        input.Email,
		"password":     input.Password,
		"phone_no":     input.PhoneNo,
		"date_of_birth": input.DateOfBirth,
		"gender":       input.Gender,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("create_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.User
	if err = json.Unmarshal(responseBody, &user); err != nil {
		logger.Error("create_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, svcerrors.Internal("invalid createUser lambda response", err)
	}

	return &user, nil
}

func (s *serviceImpl) Login(ctx context.Context, email string, password string) (string, error) {
	lambdaName := getLambdaName("LAMBDA_LOGIN_NAME", "login")
	payload := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("login_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return "", err
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		logger.Error("login_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return "", svcerrors.Internal("invalid login lambda response", err)
	}
	if response.Token == "" {
		return "", svcerrors.Validation("login response did not include token")
	}
	return response.Token, nil
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
	mergeAuthorization(ctx, payload)

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("delete_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.User
	if err = json.Unmarshal(responseBody, &user); err != nil {
		logger.Error("delete_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, svcerrors.Internal("invalid deleteUser lambda response", err)
	}

	return &user, nil
}

func (s *serviceImpl) UpdateUser(ctx context.Context, id string, input UpdateUserParams) (*domain.User, error) {
	idInt, err := validator.ParsePositiveIntID(id)
	if err != nil {
		return nil, err
	}

	lambdaName := getLambdaName("LAMBDA_UPDATE_USER_NAME", "updateUser")

	payload := map[string]interface{}{
		"id":             idInt,
		"username":       input.Username,
		"email":          input.Email,
		"phone_no":       input.PhoneNo,
		"date_of_birth":  input.DateOfBirth,
		"gender":         input.Gender,
	}
	mergeAuthorization(ctx, payload)

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		logger.Error("update_user_lambda_invocation_failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	var user domain.User
	if err = json.Unmarshal(responseBody, &user); err != nil {
		logger.Error("update_user_response_unmarshal_failed", map[string]any{"error": err.Error()})
		return nil, svcerrors.Internal("invalid updateUser lambda response", err)
	}

	return &user, nil
}

func getLambdaName(envVar string, fallback string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return fallback
}
