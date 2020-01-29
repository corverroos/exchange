package results

import (
	"context"
	"database/sql"
	"encoding/json"
	"exchange/matcher"
	"strings"
	"time"
)

func Create(ctx context.Context, dbc *sql.DB, r matcher.Result) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	b, err := json.Marshal(r.Trades)
	if err != nil {
		return 0, err
	}

	q.WriteString("insert into results set `created_at`=? ")
	args = append(args, time.Now())

	q.WriteString(", `seq`=?")
	args = append(args, r.Command.Sequence)

	q.WriteString(", `type`=?")
	args = append(args, r.Type)

q.WriteString(", `order_id`=?")
	args = append(args, r.Command.OrderID)

	q.WriteString(", `trades_json`=?")
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

	notify, err := events.Insert(ctx, tx, id, r.Type)
	if err != nil {
		return 0, err
	}
	defer notify()

	return id , tx.Commit()
}

func LookupLast(ctx context.Context, dbc *sql.DB) (*Result, error) {
	return lookupWhere(ctx, dbc, "true order by id desc limit 1")
}


func ListAll(ctx context.Context, dbc *sql.DB) ([]Result, error) {
	return listWhere(ctx, dbc, "true")
}
