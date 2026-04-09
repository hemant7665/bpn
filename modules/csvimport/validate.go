// Package csvimport parses import CSV rows and validates them (see README for column contract).
package csvimport

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	svcerrors "project-serverless/internal/errors"
)

const (
	HeaderFullName    = "full_name"
	HeaderEmail       = "email"
	HeaderAge         = "age"
	HeaderPhoneNo     = "phone_no"
	HeaderDateOfBirth = "date_of_birth"
	HeaderGender      = "gender"

	dobLayout = "2006-01-02"
)

var genderAllowed = map[string]struct{}{
	"male":   {},
	"female": {},
	"others": {},
}

var phoneDigitsOnly = regexp.MustCompile(`^[0-9]{10}$`)

// RowError describes a validation failure for one data row (1-based row index includes header as row 1).
type RowError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// Report is written to S3 as JSON after processing.
type Report struct {
	Total  int        `json:"total"`
	Passed int        `json:"passed"`
	Failed int        `json:"failed"`
	Errors []RowError `json:"errors"`
}

var requiredHeaders = []string{
	HeaderFullName,
	HeaderEmail,
	HeaderAge,
	HeaderPhoneNo,
	HeaderDateOfBirth,
	HeaderGender,
}

// ParseAndValidateCSV reads CSV with required headers (case-insensitive). Returns a report (even if parse fails entirely).
func ParseAndValidateCSV(r io.Reader) (*Report, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, svcerrors.ImportInternal("csv read failed", err)
	}
	rep := &Report{Errors: []RowError{}}
	if len(rows) == 0 {
		return rep, svcerrors.ImportValidation("empty csv")
	}
	header := rows[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, name := range requiredHeaders {
		if _, ok := idx[name]; !ok {
			return nil, svcerrors.ImportValidation(fmt.Sprintf("missing required header %q", name))
		}
	}
	dataRows := rows[1:]
	rep.Total = len(dataRows)
	now := time.Now().UTC()
	for i, row := range dataRows {
		lineNo := i + 2 // 1-based; row 1 is header
		if err := validateRow(row, idx, lineNo, now); err != nil {
			rep.Failed++
			rep.Errors = append(rep.Errors, RowError{Row: lineNo, Reason: err.Error()})
			continue
		}
		rep.Passed++
	}
	return rep, nil
}

func cell(row []string, idx map[string]int, col string) string {
	j := idx[col]
	if j < 0 || j >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[j])
}

func validateRow(row []string, idx map[string]int, lineNo int, now time.Time) error {
	full := cell(row, idx, HeaderFullName)
	if full == "" {
		return svcerrors.PlainMessage("full_name is required")
	}
	email := cell(row, idx, HeaderEmail)
	if email == "" {
		return svcerrors.PlainMessage("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return svcerrors.PlainMessage("email must be a valid address")
	}
	ageStr := cell(row, idx, HeaderAge)
	if ageStr == "" {
		return svcerrors.PlainMessage("age is required")
	}
	age, err := strconv.Atoi(ageStr)
	if err != nil || age <= 0 {
		return svcerrors.PlainMessage("age must be a positive integer")
	}

	phone := cell(row, idx, HeaderPhoneNo)
	if phone == "" {
		return svcerrors.PlainMessage("phone_no is required")
	}
	if !phoneDigitsOnly.MatchString(phone) {
		return svcerrors.PlainMessage("phone_no must be exactly 10 digits with no letters or other characters")
	}

	dobStr := cell(row, idx, HeaderDateOfBirth)
	if dobStr == "" {
		return svcerrors.PlainMessage("date_of_birth is required")
	}
	birth, err := time.ParseInLocation(dobLayout, dobStr, time.UTC)
	if err != nil {
		return svcerrors.PlainMessage("date_of_birth must be YYYY-MM-DD")
	}
	endOfBirthDay := time.Date(birth.Year(), birth.Month(), birth.Day(), 23, 59, 59, 999999999, time.UTC)
	if endOfBirthDay.After(now) {
		return svcerrors.PlainMessage("date_of_birth must not be in the future")
	}
	oldest := now.AddDate(-120, 0, 0)
	if birth.Before(oldest) {
		return svcerrors.PlainMessage("date_of_birth must not be more than 120 years ago")
	}

	g := strings.ToLower(strings.TrimSpace(cell(row, idx, HeaderGender)))
	if g == "" {
		return svcerrors.PlainMessage("gender is required")
	}
	if _, ok := genderAllowed[g]; !ok {
		return svcerrors.PlainMessage("gender must be one of: male, female, others")
	}

	return nil
}
