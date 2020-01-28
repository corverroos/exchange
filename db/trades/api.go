package trades

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type CreateReq struct {
	IsBuy        bool
	Seq          int64
	SeqIdx       int
	Price        decimal.Decimal
	Volume       decimal.Decimal
	MakerOrderID int64
	TakerOrderID int64
}

func Create(ctx context.Context, dbc *sql.DB, req CreateReq) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("insert into trades set `created_at`=? ")
	args = append(args, time.Now())

	q.WriteString(", `is_buy`=?")
	args = append(args, req.IsBuy)

	q.WriteString(", `seq`=?")
	args = append(args, req.Seq)

	q.WriteString(", `seq_idx`=?")
	args = append(args, req.SeqIdx)

	q.WriteString(", `price`=?")
	args = append(args, req.Price)

	q.WriteString(", `volume`=?")
	args = append(args, req.Volume)

	q.WriteString(", `maker_order_id`=?")
	args = append(args, req.MakerOrderID)

	q.WriteString(", `taker_order_id`=?")
	args = append(args, req.TakerOrderID)

	res, err := dbc.ExecContext(ctx, q.String(), args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}
