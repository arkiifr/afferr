package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type DB struct { SQL *sql.DB; Path string }

func DefaultPath() string { home, err := os.UserHomeDir(); if err != nil { return "affer.db" }; return filepath.Join(home, ".local", "share", "affer", "affer.db") }
func Open(path string) (*DB, error) { if path == "" { path = DefaultPath() }; if path != ":memory:" { if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return nil, err } }; sqlDB, err := sql.Open("sqlite", path); if err != nil { return nil, err }; db := &DB{SQL: sqlDB, Path: path}; if err := db.Migrate(context.Background()); err != nil { sqlDB.Close(); return nil, err }; return db, nil }
func (db *DB) Close() error { if db == nil || db.SQL == nil { return nil }; return db.SQL.Close() }
func (db *DB) Migrate(ctx context.Context) error { if db == nil || db.SQL == nil { return errors.New("nil database") }; schema, err := schemaFS.ReadFile("schema.sql"); if err != nil { return err }; _, err = db.SQL.ExecContext(ctx, string(schema)); return err }
func (db *DB) Ping(ctx context.Context) error { if db == nil || db.SQL == nil { return errors.New("nil database") }; return db.SQL.PingContext(ctx) }
func utcNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func checkDB(db *DB) error { if db == nil || db.SQL == nil { return fmt.Errorf("database is not open") }; return nil }
