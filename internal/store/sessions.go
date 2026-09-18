package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

type Session struct { ID string; Project string; Title string; Model string; Created time.Time }
type StoredMessage struct { ID int64; Session string; Role string; Content string; Tokens int; Created time.Time }

type StoredToolCall struct { ID, Session, Tool, Args, Result string; Cost float64; Created time.Time }

func NewID() string { buf := make([]byte, 12); if _, err := rand.Read(buf); err != nil { return time.Now().UTC().Format("20060102T150405.000000000") }; return hex.EncodeToString(buf) }
func (db *DB) CreateSession(ctx context.Context, project, title, model string) (Session, error) { if err := checkDB(db); err != nil { return Session{}, err }; s := Session{ID: NewID(), Project: project, Title: title, Model: model, Created: time.Now().UTC()}; _, err := db.SQL.ExecContext(ctx, `INSERT INTO sessions(id, project, title, model, created) VALUES(?,?,?,?,?)`, s.ID, s.Project, s.Title, s.Model, s.Created.Format(time.RFC3339Nano)); return s, err }
func (db *DB) GetSession(ctx context.Context, id string) (Session, error) { if err := checkDB(db); err != nil { return Session{}, err }; var s Session; var created string; err := db.SQL.QueryRowContext(ctx, `SELECT id, project, title, model, created FROM sessions WHERE id=?`, id).Scan(&s.ID, &s.Project, &s.Title, &s.Model, &created); if err != nil { return s, err }; s.Created, _ = time.Parse(time.RFC3339Nano, created); return s, nil }
func (db *DB) ListSessions(ctx context.Context, project string) ([]Session, error) { if err := checkDB(db); err != nil { return nil, err }; query, args := `SELECT id, project, title, model, created FROM sessions ORDER BY created DESC`, []any{}; if project != "" { query = `SELECT id, project, title, model, created FROM sessions WHERE project=? ORDER BY created DESC`; args = []any{project} }; rows, err := db.SQL.QueryContext(ctx, query, args...); if err != nil { return nil, err }; defer rows.Close(); var out []Session; for rows.Next() { var s Session; var created string; if err := rows.Scan(&s.ID, &s.Project, &s.Title, &s.Model, &created); err != nil { return nil, err }; s.Created, _ = time.Parse(time.RFC3339Nano, created); out = append(out, s) }; return out, rows.Err() }
func (db *DB) SetSessionModel(ctx context.Context, id, model string) error { if err := checkDB(db); err != nil { return err }; _, err := db.SQL.ExecContext(ctx, `UPDATE sessions SET model=? WHERE id=?`, model, id); return err }
func (db *DB) AppendMessage(ctx context.Context, session, role, content string, tokens int) error { if err := checkDB(db); err != nil { return err }; _, err := db.SQL.ExecContext(ctx, `INSERT INTO messages(session, role, content, tokens, created) VALUES(?,?,?,?,?)`, session, role, content, tokens, utcNow()); return err }
func (db *DB) Messages(ctx context.Context, session string) ([]StoredMessage, error) { if err := checkDB(db); err != nil { return nil, err }; rows, err := db.SQL.QueryContext(ctx, `SELECT id, session, role, content, tokens, created FROM messages WHERE session=? ORDER BY id`, session); if err != nil { return nil, err }; defer rows.Close(); var out []StoredMessage; for rows.Next() { var m StoredMessage; var created string; if err := rows.Scan(&m.ID, &m.Session, &m.Role, &m.Content, &m.Tokens, &created); err != nil { return nil, err }; m.Created, _ = time.Parse(time.RFC3339Nano, created); out = append(out, m) }; return out, rows.Err() }
func (db *DB) AppendToolCall(ctx context.Context, call StoredToolCall) error { if err := checkDB(db); err != nil { return err }; if call.ID == "" { call.ID = NewID() }; _, err := db.SQL.ExecContext(ctx, `INSERT INTO tool_calls(id, session, tool, args, result, cost, created) VALUES(?,?,?,?,?,?,?)`, call.ID, call.Session, call.Tool, call.Args, call.Result, call.Cost, utcNow()); return err }
func (db *DB) LastUsedModel(ctx context.Context, project string) (string, error) { if err := checkDB(db); err != nil { return "", err }; var model string; err := db.SQL.QueryRowContext(ctx, `SELECT model FROM sessions WHERE project=? AND model<>'' ORDER BY created DESC LIMIT 1`, project).Scan(&model); if errors.Is(err, sql.ErrNoRows) { return "", nil }; return model, err }
