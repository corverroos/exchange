package trades

import (
	"time"

	"github.com/shopspring/decimal"
)

type Trade struct {
	ID           int64
	IsBuy        bool
	Seq          int64 // Sequence of the matcher command producing this trade.
	SeqIdx       int   // Index of this trade in the sequence's set of trades.
	Price        decimal.Decimal
	Volume       decimal.Decimal
	MakerOrderID int64
	TakerOrderID int64
	CreatedAt    time.Time
}
