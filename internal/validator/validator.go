package validator

import (
	"strconv"

	svcerrors "project-serverless/internal/errors"

	"github.com/go-playground/validator/v10"
)

var v = validator.New()

func ValidateStruct(s any) error {
	return v.Struct(s)
}

func ParsePositiveIntID(raw any) (int, error) {
	switch v := raw.(type) {
	case int:
		if v <= 0 {
			return 0, svcerrors.Validation("id must be greater than 0")
		}
		return v, nil
	case float64:
		id := int(v)
		if id <= 0 || float64(id) != v {
			return 0, svcerrors.Validation("id must be a positive integer")
		}
		return id, nil
	case string:
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return 0, svcerrors.Validation("id must be a positive integer")
		}
		return id, nil
	default:
		return 0, svcerrors.Validation("id must be numeric")
	}
}
