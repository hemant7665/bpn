package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractAnonymousMutationFirstField(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{`mutation { login(email: "a", password: "b") { token } }`, "login"},
		{`mutation{createUser(input:{username:"u",email:"e",password:"p"}){id}}`, "createUser"},
		{`mutation  {  login (`, "login"},
		{`query { getUser(id: "1") { id } }`, ""},
		{`mutation Named { login { token } }`, ""},
	}
	for _, tc := range tests {
		got := extractAnonymousMutationFirstField(tc.query)
		if got != tc.want {
			t.Errorf("extractAnonymousMutationFirstField(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestGraphqlShouldSkipAuth_AnonymousLogin(t *testing.T) {
	skip := map[string]struct{}{"login": {}, "createuser": {}}
	body := `{"query":"mutation { login(email: \"a@b.com\", password: \"password12345\") { token } }"}`
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	got, ok := graphqlShouldSkipAuth(req, skip)
	if !ok {
		t.Fatal("graphqlShouldSkipAuth: expected ok")
	}
	if !got {
		t.Fatal("expected auth skipped for anonymous login mutation")
	}
}

func TestGraphqlShouldSkipAuth_AnonymousGetUserRequiresAuth(t *testing.T) {
	skip := map[string]struct{}{"login": {}, "createuser": {}}
	body := `{"query":"mutation { getUser(id: \"1\") { id } }"}`
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	got, ok := graphqlShouldSkipAuth(req, skip)
	if !ok {
		t.Fatal("graphqlShouldSkipAuth: expected ok")
	}
	if got {
		t.Fatal("expected auth required when first field is not in skip list")
	}
}
