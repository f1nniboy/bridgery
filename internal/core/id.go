package core

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

type CID string

var entropy = &ulid.LockedMonotonicReader{
	MonotonicReader: ulid.Monotonic(rand.Reader, 0),
}

func NewID() CID {
	return CID(ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String())
}
