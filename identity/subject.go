package identity

import "context"

type ContextKey string

var (
	SubjectKey = ContextKey("subject")
)

func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, SubjectKey, subject)
}

func ExtractSubject(ctx context.Context) string {
	subject := ctx.Value(SubjectKey)
	if subject != nil {
		return subject.(string)
	}

	return ""
}
