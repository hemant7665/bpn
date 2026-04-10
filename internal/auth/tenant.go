package auth

import (
	"context"
	"strings"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/repository"
)

// ResolveTenant validates the Bearer JWT and loads the user's tenant_id from write_model.users.
func ResolveTenant(ctx context.Context, authorization string, users repository.UserRepository) (tenantID string, userID int, err error) {
	if strings.TrimSpace(authorization) == "" {
		return "", 0, svcerrors.Unauthorized("unauthorized")
	}
	claims, err := AuthorizeHeader(authorization)
	if err != nil {
		return "", 0, svcerrors.Unauthorized("unauthorized")
	}
	u, err := users.GetWriteUserByID(ctx, claims.UserID)
	if err != nil {
		return "", 0, svcerrors.NotFound("user not found")
	}
	return domain.NormalizeTenantID(u.TenantID), u.ID, nil
}
