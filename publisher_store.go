package main

import (
	"context"
	"database/sql"
)

// publicationPosts streams one consistent view of published posts. The consumer
// controls backpressure, so a generation never retains every Markdown body.
func (s *Store) publicationPosts(ctx context.Context, consume func(Post) error) error {
	opts := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	if s.dialect == "sqlite" {
		opts = &sql.TxOptions{ReadOnly: true}
	}
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT "+postColumns+" FROM news WHERE status=1 ORDER BY id DESC")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return err
		}
		if err := consume(p); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return tx.Commit()
}
