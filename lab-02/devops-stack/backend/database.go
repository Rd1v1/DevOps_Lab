package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"notes/internal/api"
)

type postgresStore struct {
	pool        *pgxpool.Pool
	schemaMu    sync.Mutex
	schemaReady bool
}

func initDB(ctx context.Context) (*postgresStore, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			return nil, errors.New("DB_PASSWORD or DATABASE_URL is required")
		}
		address := &url.URL{Scheme: "postgres", Host: "db:5432", Path: "/notes", User: url.UserPassword("notes", password)}
		dsn = address.String()
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("cannot create database pool")
	}
	return &postgresStore{pool: pool}, nil
}

func (s *postgresStore) ensureSchema(ctx context.Context) error {
	s.schemaMu.Lock()
	defer s.schemaMu.Unlock()
	if s.schemaReady {
		return nil
	}
	_, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS notes (
        id BIGSERIAL PRIMARY KEY,
        title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
        body TEXT NOT NULL DEFAULT ''
    )`)
	if err != nil {
		return fmt.Errorf("schema unavailable: %w", err)
	}
	s.schemaReady = true
	return nil
}

func (s *postgresStore) Health(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return err
	}
	return s.ensureSchema(ctx)
}

func (s *postgresStore) List(ctx context.Context) ([]api.Note, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, "SELECT id, title, body FROM notes ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []api.Note{}
	for rows.Next() {
		var note api.Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Body); err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func (s *postgresStore) Create(ctx context.Context, note api.Note) (int64, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return 0, err
	}
	var id int64
	err := s.pool.QueryRow(ctx, "INSERT INTO notes (title, body) VALUES ($1, $2) RETURNING id", note.Title, note.Body).Scan(&id)
	return id, err
}

func (s *postgresStore) Get(ctx context.Context, id int64) (api.Note, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return api.Note{}, err
	}
	var note api.Note
	err := s.pool.QueryRow(ctx, "SELECT id, title, body FROM notes WHERE id=$1", id).Scan(&note.ID, &note.Title, &note.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return note, api.ErrNotFound
	}
	return note, err
}
