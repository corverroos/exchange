package results

import (
	"exchange/matcher"
	"time"
)

type Result struct {
	ID        int64
	Seq       int64 // Sequence of the matcher command producing this trade.
	Type      matcher.Type
	OrderID   int64
	CreatedAt time.Time
	Trades    []matcher.Trade
}
