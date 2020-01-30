package orders

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/shift"
	"github.com/shopspring/decimal"
)

//go:generate shiftgen -inserter=CreateReq -updaters=postReq,cancelReq,completeReq -table=orders

var (
	events = rsql.NewEventsTableInt("order_events",
		rsql.WithEventsInMemNotifier(),
		rsql.WithEventMetadataField("metadata"))

	fsm = shift.NewFSM(events, shift.WithMetadata()).
		Insert(StatusPending, CreateReq{}, StatusComplete, StatusCancelling, StatusPosted).
		Update(StatusPosted, postReq{}, StatusComplete, StatusCancelling).
		Update(StatusCancelling, cancelReq{}, StatusComplete).
		Update(StatusComplete, completeReq{}).Build()
)

type (
	CreateReq struct {
		Type  Type
		IsBuy bool

		LimitVolume decimal.Decimal
		LimitPrice  decimal.Decimal

		MarketBase    decimal.Decimal
		MarketCounter decimal.Decimal
	}

	cancelReq struct {
		ID    int64
		isBuy bool // Only for metadata
	}

	postReq struct {
		ID        int64
		UpdateSeq int64
	}

	completeReq struct {
		ID        int64
		UpdateSeq int64
	}
)

func ToStream(dbc *sql.DB) reflex.StreamFunc {
	return events.ToStream(dbc)
}

func FillGaps(dbc *sql.DB) {
	rsql.FillGaps(dbc, events)
}

func (r cancelReq) GetMetadata(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) ([]byte, error) {
	return json.Marshal(&r.isBuy)
}

func (r postReq) GetMetadata(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) ([]byte, error) {
	return nil, nil
}

func (r completeReq) GetMetadata(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) ([]byte, error) {
	return nil, nil
}

func (r CreateReq) GetMetadata(ctx context.Context, tx *sql.Tx, id int64, status shift.Status) ([]byte, error) {
	return json.Marshal(&r)
}
