package observability

import "context"

type correlationKey struct{}

type Correlation struct {
	ProjectID string
	RunID     string
	Stage     string
	TaskID    string
	RequestID string
	Source    string
	Actor     string
	ActorRole string
}

func ContextWithCorrelation(ctx context.Context, correlation Correlation) context.Context {
	return context.WithValue(ctx, correlationKey{}, correlation)
}

func CorrelationFromContext(ctx context.Context) Correlation {
	if ctx == nil {
		return Correlation{}
	}
	correlation, _ := ctx.Value(correlationKey{}).(Correlation)
	return correlation
}
