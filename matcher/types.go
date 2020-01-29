package matcher

import (
	"github.com/shopspring/decimal"
)

//go:generate stringer -type=CommandType -trimprefix=Command

type CommandType int

const (
	CommandUnknown  CommandType = 0
	CommandLimit    CommandType = 1
	CommandMarket   CommandType = 2
	CommandPostOnly CommandType = 3
	CommandCancel   CommandType = 4
)

type Command struct {
	Sequence int64
	Type     CommandType
	IsBuy    bool
	OrderID  int64

	LimitPrice  decimal.Decimal
	LimitVolume decimal.Decimal

	MarketBase    decimal.Decimal // Eg. when buying BTC with X USD
	MarketCounter decimal.Decimal // Eg. when selling X BTC for USD
}

// Order is a bid or ask order.
type Order struct {
	ID        int64
	Price     decimal.Decimal
	Remaining decimal.Decimal // Counter remaining
}

// Base returns the equivalent base amount; price * remaining.
func (o Order) Base() decimal.Decimal {
	return o.Price.Mul(o.Remaining)
}

type OrderBook struct {
	Sequence int64
	Bids     []Order
	Asks     []Order
}

type Trade struct {
	MakerOrderID int64
	TakerOrderID int64
	MakerFilled  bool
	Volume       decimal.Decimal
	Price        decimal.Decimal
	IsBuy        bool
}

//go:generate stringer -type=Type -trimprefix=Type

type Type int

func (t Type) ReflexType() int {
	return int(t)
}

const (
	TypeUnknown        Type = 0
	TypeCommandOld     Type = 1
	TypeCommandUnknown Type = 2
	TypeCancelFailed   Type = 3
	TypeCancelled      Type = 4
	TypePostFailed     Type = 5
	TypePosted         Type = 6
	TypeMarketEmpty    Type = 7
	TypeMarketPartial  Type = 8
	TypeMarketFull     Type = 9
	TypeLimitTaker     Type = 10
	TypeLimitPartial   Type = 11
	TypeLimitMaker     Type = 12
)

type Result struct {
	Type    Type
	Trades  []Trade
	Command Command
}
