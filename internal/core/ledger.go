package core

type Ledger interface {
	Put(id CID, m Message)
	Get(id CID) (Message, bool)
	Delete(id CID)

	Bind(id CID, addr Address, nativeIDs []string)
	Canonical(addr Address, nativeID string) (CID, bool)
	Native(id CID, dst Address) ([]string, bool)

	Close() error
}
