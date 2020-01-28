package orders

import (
	"context"
	"database/sql"

	"github.com/luno/jettison/errors"

	"github.com/shopspring/decimal"
)

func CreateLimit(ctx context.Context, dbc *sql.DB, isBuy bool, price, volume decimal.Decimal,
	isPostOnly bool) (int64, error) {

	typ := TypeLimit
	if isPostOnly {
		typ = TypePostOnly
	}

	return fsm.Insert(ctx, dbc, createReq{
		IsBuy:       isBuy,
		Type:        typ,
		LimitVolume: volume,
		LimitPrice:  price,
	})
}

func CreateMarketSell(ctx context.Context, dbc *sql.DB, counter decimal.Decimal) (int64, error) {
	return fsm.Insert(ctx, dbc, createReq{
		Type:          TypeMarket,
		IsBuy:         false,
		MarketCounter: counter,
	})
}

func CreateMarketBuy(ctx context.Context, dbc *sql.DB, base decimal.Decimal) (int64, error) {
	return fsm.Insert(ctx, dbc, createReq{
		Type:       TypeMarket,
		IsBuy:      true,
		MarketBase: base,
	})
}
func RequestCancel(ctx context.Context, dbc *sql.DB, id int64) error {
	o, err := Lookup(ctx, dbc, id)
	if err != nil {
		return err
	}

	if o.Status == StatusComplete {
		return errors.New("cannot cancel complete order")
	}

	err = fsm.Update(ctx, dbc, o.Status, StatusCancelling, cancelReq{ID: id})
	if err != nil {
		return errors.Wrap(err, "cancelling error")
	}

	return nil
}

func UpdatePosted(ctx context.Context, dbc *sql.DB, id int64, seq int64) error {
	o, err := Lookup(ctx, dbc, id)
	if err != nil {
		return err
	}

	if o.UpdateSeq >= seq {
		// This sequence was already processed.
		return nil
	}

	if o.Status == StatusCancelling {
		// Skip posted if cancelling
		return nil
	}

	r := postReq{
		ID:        id,
		UpdateSeq: seq,
	}

	err = fsm.Update(ctx, dbc, o.Status, StatusPosted, r)
	if err != nil {
		return errors.Wrap(err, "posted error")
	}

	return nil
}

func Complete(ctx context.Context, dbc *sql.DB, id int64, seq int64) error {
	o, err := Lookup(ctx, dbc, id)
	if err != nil {
		return err
	}

	if o.UpdateSeq >= seq {
		// This sequence was already processed.
		return nil
	}

	r := completeReq{
		ID:        id,
		UpdateSeq: seq,
	}

	err = fsm.Update(ctx, dbc, o.Status, StatusComplete, r)
	if err != nil {
		return errors.Wrap(err, "complete error")
	}

	return nil
}

func ScanAll(ctx context.Context, dbc *sql.DB, fn func(*Order) error) error {
	return scanWhere(ctx, dbc, fn, "true")
}
