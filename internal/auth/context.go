package auth

import "context"

type contextKey struct {
	name string
}

var (
	ctxKeyAuthorization = contextKey{"authorizationHeader"}
	ctxKeyClaims        = contextKey{"jwtClaims"}
)

// WithAuthorization stores the raw Authorization header value (e.g. "Bearer <jwt>") for downstream Lambda calls.
func WithAuthorization(ctx context.Context, authorizationHeader string) context.Context {
	return context.WithValue(ctx, ctxKeyAuthorization, authorizationHeader)
}

// AuthorizationFromContext returns the Authorization header value attached by GraphQL auth middleware.
func AuthorizationFromContext(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyAuthorization).(string)
	return s
}

// WithClaims stores validated JWT claims (optional; for resolver-level checks later).
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ctxKeyClaims, claims)
}
