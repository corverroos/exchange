package orders

import (
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID     int64
	Type   Type
	IsBuy  bool
	Status Status

	LimitVolume decimal.Decimal
	LimitPrice  decimal.Decimal

	MarketBase    decimal.Decimal // Buying counter with X base
	MarketCounter decimal.Decimal // Selling X counter for base

	CreatedAt time.Time
	UpdatedAt time.Time
	// UpdateSeq is the last match command result sequence
	// that update this Order.
	UpdateSeq int64
}

type Type int

const (
	TypeUnknown  Type = 0
	TypeLimit    Type = 1
	TypeMarket   Type = 2
	TypePostOnly Type = 3
)

type Status int

func (s Status) ShiftStatus() int {
	return int(s)
}

func (s Status) ReflexType() int {
	return int(s)
}

const (
	StatusUnknown    Status = 0
	StatusPending    Status = 1
	StatusPosted     Status = 2
	StatusCancelling Status = 4
	StatusComplete   Status = 5
)
