package store

import (
	"context"
	"time"
)

type Usage struct { Day string; Model string; Input int; Output int; Cost float64 }
func (db *DB) RecordUsage(ctx context.Context, model string, input, output int, cost float64) error { if err := checkDB(db); err != nil { return err }; day := time.Now().UTC().Format("2006-01-02"); _, err := db.SQL.ExecContext(ctx, `INSERT INTO usage(day, model, input, output, cost) VALUES(?,?,?,?,?) ON CONFLICT(day,model) DO UPDATE SET input=input+excluded.input, output=output+excluded.output, cost=cost+excluded.cost`, day, model, input, output, cost); return err }
func (db *DB) Usage(ctx context.Context, since time.Time) ([]Usage, error) { if err := checkDB(db); err != nil { return nil, err }; rows, err := db.SQL.QueryContext(ctx, `SELECT day, model, input, output, cost FROM usage WHERE day>=? ORDER BY day DESC, model`, since.UTC().Format("2006-01-02")); if err != nil { return nil, err }; defer rows.Close(); var out []Usage; for rows.Next() { var u Usage; if err := rows.Scan(&u.Day, &u.Model, &u.Input, &u.Output, &u.Cost); err != nil { return nil, err }; out = append(out, u) }; return out, rows.Err() }
func (db *DB) Audit(ctx context.Context, session, tool, decision, args string) error { if err := checkDB(db); err != nil { return err }; _, err := db.SQL.ExecContext(ctx, `INSERT INTO audits(ts, tool, decision, session, args) VALUES(?,?,?,?,?)`, utcNow(), tool, decision, session, args); return err }
