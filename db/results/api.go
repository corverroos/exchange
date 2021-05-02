package results

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/corverroos/exchange/matcher"
	"strings"
	"time"
)

const Cursor = "results"

func Create(ctx context.Context, dbc *sql.DB, rl []matcher.Result) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	b, err := json.Marshal(rl)
	if err != nil {
		return 0, err
	}

	var start, end int64
	if len(rl) > 0 {
		start = rl[0].Sequence
		end = rl[len(rl)-1].Sequence
	}

	q.WriteString("insert into results set `created_at`=? ")
	args = append(args, time.Now())

	q.WriteString(", `start_seq`=?")
	args = append(args, start)

	q.WriteString(", `end_seq`=?")
	args = append(args, end)

	q.WriteString(", `results_json`=?")
	args = append(args, b)

	tx, err := dbc.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, q.String(), args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	notify, err := events.Insert(ctx, tx, id, etype{})
	if err != nil {
		return 0, err
	}
	defer notify()

	return id, tx.Commit()
}

type etype struct{}

func (e etype) ReflexType() int {
	return 1
}

func LookupLast(ctx context.Context, dbc *sql.DB) (*Result, error) {
	return lookupWhere(ctx, dbc, "true order by id desc limit 1")
}

func ListAll(ctx context.Context, dbc *sql.DB) ([]Result, error) {
	return listWhere(ctx, dbc, "true")
}
