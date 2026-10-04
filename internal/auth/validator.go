package auth

import "context"

type Validator interface {
	ResolveUserID(ctx context.Context, plainToken string) (string, error)
}
