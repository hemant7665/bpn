package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"project-serverless/internal/domain"
)

type Service interface {
	GetUser(ctx context.Context, id string) (*domain.User, error)
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
		log.Println("Initializing admin orchestrator for LocalStack environment")
		awsEndpoint := os.Getenv("AWS_LAMBDA_ENDPOINT")
		if awsEndpoint == "" {
			awsEndpoint = os.Getenv("AWS_ENDPOINT_URL")
		}

		if awsEndpoint == "" {
			return nil, errors.New("AWS_ENDPOINT_URL or AWS_LAMBDA_ENDPOINT must be set in local environment")
		}

		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: awsEndpoint, SigningRegion: "us-east-1"}, nil
				},
			)),
		)
	} else {
		log.Println("Initializing admin orchestrator for AWS environment: " + environment)
		cfg, err = config.LoadDefaultConfig(ctx)
	}

	if err != nil {
		log.Println("Failed to load AWS config for environment: " + environment + ", error: " + err.Error())
		return nil, errors.New("service unavailable")
	}

	log.Println("Admin orchestrator initialized successfully")
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
		return nil, err
	}

	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payloadBytes,
	}

	result, err := s.lambdaClient.Invoke(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.FunctionError != nil {
		return nil, errors.New("lambda function error: " + *result.FunctionError)
	}

	return result.Payload, nil
}

func (s *serviceImpl) GetUser(ctx context.Context, id string) (*domain.User, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.New("invalid user id: must be an integer")
	}

	lambdaName := os.Getenv("LAMBDA_NAME")
	if lambdaName == "" {
		lambdaName = "getUser"
	}

	payload := map[string]interface{}{
		"id": idInt,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		log.Println("GetUser lambda invocation failed")
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		log.Println("Failed to unmarshal GetUser lambda response")
		return nil, err
	}

	return &user, nil

}

func (s *serviceImpl) CreateUser(ctx context.Context, name string, email string) (*domain.User, error) {

	lambdaName := os.Getenv("LAMBDA_NAME")
	if lambdaName == "" {
		lambdaName = "createUser"
	}

	payload := map[string]interface{}{
		"name":  name,
		"email": email,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		log.Println("CreateUser lambda invocation failed")
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		log.Println("Failed to unmarshal CreateUser lambda response")
		return nil, err
	}

	return &user, nil
}

func (s *serviceImpl) DeleteUser(ctx context.Context, id string) (*domain.User, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.New("invalid user id: must be an integer")
	}

	lambdaName := os.Getenv("LAMBDA_NAME")
	if lambdaName == "" {
		lambdaName = "deleteUser"
	}

	payload := map[string]interface{}{
		"id": idInt,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		log.Println("DeleteUser lambda invocation failed")
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		log.Println("Failed to unmarshal DeleteUser lambda response")
		return nil, err
	}

	return &user, nil

}

func (s *serviceImpl) UpdateUser(ctx context.Context, id string, name string, email string) (*domain.User, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.New("invalid user id: must be an integer")
	}

	lambdaName := os.Getenv("LAMBDA_NAME")
	if lambdaName == "" {
		lambdaName = "updateUser"
	}

	payload := map[string]interface{}{
		"id":    idInt,
		"name":  name,
		"email": email,
	}

	responseBody, err := s.invokeLambda(ctx, lambdaName, payload)
	if err != nil {
		log.Println("UpdateUser lambda invocation failed")
		return nil, err
	}

	var user domain.User
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		log.Println("Failed to unmarshal UpdateUser lambda response")
		return nil, err
	}

	return &user, nil

}
