package logctx

import "context"

type ctxKey struct{}

// WithRequestID stores a request correlation ID in ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestIDFrom returns the request correlation ID from ctx, or "".
func RequestIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}
