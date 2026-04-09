package csvimport

import (
	"strings"
	"testing"
	"time"
)

func csvLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func TestParseAndValidateCSV_ok(t *testing.T) {
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		"Jane Doe,jane@example.com,25,9876543210,2000-01-15,female",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 || rep.Passed != 1 || rep.Failed != 0 {
		t.Fatalf("unexpected counts: %+v", rep)
	}
}

func TestParseAndValidateCSV_rowErrors(t *testing.T) {
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		",bad@,0,123,2000-01-01,male",
		"Bob,bob@example.com,3,9876543210,2020-05-01,male",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 2 || rep.Passed != 1 || rep.Failed != 1 {
		t.Fatalf("unexpected counts: %+v", rep)
	}
	if len(rep.Errors) != 1 {
		t.Fatalf("expected 1 row error, got %d", len(rep.Errors))
	}
}

func TestParseAndValidateCSV_missingHeader(t *testing.T) {
	csv := "a,b,c\n1,2,3\n"
	_, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for missing headers")
	}
}

func TestValidatePhone(t *testing.T) {
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		"A,a@b.co,20,98765abcde,2004-06-01,male",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || !strings.Contains(rep.Errors[0].Reason, "10 digits") {
		t.Fatalf("expected phone error, got %+v", rep)
	}
}

func TestValidateGender(t *testing.T) {
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		"A,a@b.co,20,9876543210,2004-06-01,unknown",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || !strings.Contains(rep.Errors[0].Reason, "gender") {
		t.Fatalf("expected gender error, got %+v", rep)
	}
}

func TestValidateGenderCaseInsensitive(t *testing.T) {
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		"A,a@b.co,20,9876543210,2004-06-01,FEMALE",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Passed != 1 {
		t.Fatalf("expected pass, got %+v", rep)
	}
}

func TestValidateDOBOver120Years(t *testing.T) {
	old := time.Now().UTC().AddDate(-121, 0, 0).Format(dobLayout)
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		"A,a@b.co,20,9876543210,"+old+",male",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || !strings.Contains(rep.Errors[0].Reason, "120 years") {
		t.Fatalf("expected dob age error, got %+v", rep)
	}
}

func TestValidateDOBFuture(t *testing.T) {
	future := time.Now().UTC().AddDate(0, 0, 1).Format(dobLayout)
	csv := csvLines(
		"full_name,email,age,phone_no,date_of_birth,gender",
		"A,a@b.co,20,9876543210,"+future+",male",
	)
	rep, err := ParseAndValidateCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || !strings.Contains(rep.Errors[0].Reason, "future") {
		t.Fatalf("expected future dob error, got %+v", rep)
	}
}
