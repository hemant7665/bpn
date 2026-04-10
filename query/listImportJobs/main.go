package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

type request struct {
	Authorization  string `json:"authorization"`
	Skip           int    `json:"skip"`
	Limit          int    `json:"limit"`
	Status         string `json:"status"`
	CreatedAtOrder string `json:"created_at_order"`
}

// importJobListItem is the listImportJobs Lambda payload: same fields as domain.ImportJob
// but without csv_s3_key / report_s3_key so paginated responses do not expose object paths.
type importJobListItem struct {
	ID           uuid.UUID `json:"id"`
	TenantID     string    `json:"tenant_id"`
	RequestedBy  int       `json:"requested_by"`
	Status       string    `json:"status"`
	TotalRows    *int      `json:"total_rows,omitempty"`
	PassedRows   *int      `json:"passed_rows,omitempty"`
	FailedRows   *int      `json:"failed_rows,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type response struct {
	Items []importJobListItem `json:"items"`
	Total int64               `json:"total"`
}

func toImportJobListItems(jobs []domain.ImportJob) []importJobListItem {
	out := make([]importJobListItem, len(jobs))
	for i := range jobs {
		j := jobs[i]
		out[i] = importJobListItem{
			ID:           j.ID,
			TenantID:     j.TenantID,
			RequestedBy:  j.RequestedBy,
			Status:       j.Status,
			TotalRows:    j.TotalRows,
			PassedRows:   j.PassedRows,
			FailedRows:   j.FailedRows,
			ErrorMessage: j.ErrorMessage,
			CreatedAt:    j.CreatedAt,
			UpdatedAt:    j.UpdatedAt,
		}
	}
	return out
}

var (
	importSvc service.ImportJobService
	userRepo  repository.UserRepository
)

func setup() error {
	svc, err := bootstrap.SetupImportJobService()
	if err != nil {
		return err
	}
	ur, err := bootstrap.SetupUserRepository()
	if err != nil {
		return err
	}
	importSvc = svc
	userRepo = ur
	return nil
}

func HandleRequest(ctx context.Context, req request) (*response, error) {
	tenantID, _, err := auth.ResolveTenant(ctx, req.Authorization, userRepo)
	if err != nil {
		return nil, err
	}
	var st *string
	if s := strings.TrimSpace(req.Status); s != "" {
		st = &s
	}
	order := strings.ToUpper(strings.TrimSpace(req.CreatedAtOrder))
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	items, total, err := importSvc.ListJobsForTenant(ctx, tenantID, req.Skip, req.Limit, st, order)
	if err != nil {
		return nil, err
	}
	return &response{Items: toImportJobListItems(items), Total: total}, nil
}

func main() {
	logger.Info("booting_list_import_jobs", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
	defer func() {
		if r := recover(); r != nil {
			logger.Error("unhandled_panic", map[string]any{"panic": r, "stack": string(debug.Stack())})
		}
	}()
	if err := setup(); err != nil {
		logger.Error("failed_to_initialize_lambda_dependencies", map[string]any{"error": err.Error()})
		panic(err)
	}
	lambda.Start(HandleRequest)
}
