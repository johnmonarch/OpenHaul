package store

import (
	"context"
	"time"
)

func (s *Store) ExportWatch(ctx context.Context) ([]WatchEntry, error) {
	return s.ListWatch(ctx)
}

func (s *Store) RemoveWatch(ctx context.Context, typ, value string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE watchlist
		SET active = 0, updated_at = ?
		WHERE identifier_type = ? AND normalized_value = ? AND active = 1`,
		now, typ, value)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
