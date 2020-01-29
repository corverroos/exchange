package orders

import (
	"database/sql"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/shift"
	"github.com/shopspring/decimal"
)

//go:generate shiftgen -inserter=createReq -updaters=postReq,cancelReq,completeReq -table=orders

var (
	events = rsql.NewEventsTableInt("order_events", rsql.WithEventsInMemNotifier())

	fsm = shift.NewFSM(events).
		Insert(StatusPending, createReq{}, StatusComplete, StatusCancelling, StatusPosted).
		Update(StatusPosted, postReq{}, StatusComplete, StatusCancelling).
		Update(StatusCancelling, cancelReq{}, StatusComplete).
		Update(StatusComplete, completeReq{}).Build()
)

type (
	createReq struct {
		Type  Type
		IsBuy bool

		LimitVolume decimal.Decimal
		LimitPrice  decimal.Decimal

		MarketBase    decimal.Decimal
		MarketCounter decimal.Decimal
	}

	cancelReq struct {
		ID int64
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
