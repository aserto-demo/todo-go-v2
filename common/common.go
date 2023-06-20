package common

type ContextKey string

func (c ContextKey) String() string {
	return "context key " + string(c)
}

var (
	ContextKeySubject = ContextKey("subject")
)
