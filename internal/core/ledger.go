package core

import "context"

type Ledger interface {
	Put(ctx context.Context, id CID, m Message) error
	Get(ctx context.Context, id CID) (Message, bool, error)
	Delete(ctx context.Context, id CID) error

	Bind(ctx context.Context, id CID, addr Address, nativeIDs []string) error
	Canonical(ctx context.Context, addr Address, nativeID string) (CID, bool, error)
	Native(ctx context.Context, id CID, dst Address) ([]string, bool, error)

	Close() error
}
