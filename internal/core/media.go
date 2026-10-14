package core

import "context"

type Media interface {
	URL(ctx context.Context, f File) (string, error)
}
