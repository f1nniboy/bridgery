package ledger

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/f1nniboy/bridgery/internal/core"
)

//go:embed migrations/*.sql
var migrations embed.FS

type sqlite struct {
	Path string `toml:"path" comment:"database file, created if missing"`
	retention

	db *sql.DB
}

func (s *sqlite) Example() any {
	s.Path = "/var/lib/bridgery/ledger.db"
	return s
}

func (s *sqlite) Open() error {
	if s.Path == "" {
		return errors.New("backend requires path")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+s.Path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return fmt.Errorf("migrate: %w", err)
	}
	s.db = db
	return nil
}

func migrate(db *sql.DB) error {
	ctx := context.Background()
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(files) {
		return fmt.Errorf("schema version %d is newer than this build (%d)", version, len(files))
	}
	for i, name := range files[version:] {
		src, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		if err := inTx(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(src)); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			_, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version+i+1))
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func inTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *sqlite) prune(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM messages WHERE updated_at < ?", before.Unix())
	return err
}

func (s *sqlite) Put(ctx context.Context, id core.CID, m core.Message) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO messages (id, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		id, string(data), time.Now().Unix())
	return err
}

func (s *sqlite) Get(ctx context.Context, id core.CID) (core.Message, bool, error) {
	var data sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT data FROM messages WHERE id = ?", id).Scan(&data)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return core.Message{}, false, nil
	case err != nil:
		return core.Message{}, false, err
	case !data.Valid:
		return core.Message{}, false, nil // deleted
	}
	var m core.Message
	if err := json.Unmarshal([]byte(data.String), &m); err != nil {
		return core.Message{}, false, err
	}
	return m, true, nil
}

func (s *sqlite) Delete(ctx context.Context, id core.CID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE messages SET data = NULL WHERE id = ?", id)
	return err
}

func (s *sqlite) Bind(ctx context.Context, id core.CID, addr core.Address, natives []string) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO messages (id, updated_at) VALUES (?, ?)
			ON CONFLICT (id) DO UPDATE SET updated_at = excluded.updated_at`, id, time.Now().Unix()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM bindings WHERE message_id = ? AND account = ? AND channel = ?",
			id, addr.Account, addr.Channel); err != nil {
			return err
		}
		for position, native := range natives {
			if _, err := tx.ExecContext(ctx, `INSERT INTO bindings (message_id, account, channel, native_id, position) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (account, channel, native_id) DO UPDATE SET message_id = excluded.message_id, position = excluded.position`,
				id, addr.Account, addr.Channel, native, position); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *sqlite) Canonical(ctx context.Context, addr core.Address, nativeID string) (core.CID, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, "SELECT message_id FROM bindings WHERE account = ? AND channel = ? AND native_id = ?",
		addr.Account, addr.Channel, nativeID).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return core.CID(id), true, nil
}

func (s *sqlite) Native(ctx context.Context, id core.CID, dst core.Address) ([]string, bool, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT native_id FROM bindings WHERE message_id = ? AND account = ? AND channel = ? ORDER BY position",
		id, dst.Account, dst.Channel)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var natives []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, false, err
		}
		natives = append(natives, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return natives, len(natives) > 0, nil
}

func (s *sqlite) Close() error {
	return s.db.Close()
}
