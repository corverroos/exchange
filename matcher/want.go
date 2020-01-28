package matcher

import (
	"github.com/shopspring/decimal"
)

// want encapsulates a request for trading by counter or base amount
// with optional limit price.
type want interface {
	PriceLimit() decimal.Decimal
	Remaining(price decimal.Decimal) (counter decimal.Decimal)
	Fill(counter, price decimal.Decimal)
	Filled()
	IsFilled() bool
}

type wantMarketBase struct {
	remaining decimal.Decimal // Base
	scale     int
}

func (w *wantMarketBase) PriceLimit() decimal.Decimal {
	return decimal.Zero
}

func (w *wantMarketBase) Remaining(price decimal.Decimal) (counter decimal.Decimal) {
	return w.remaining.DivRound(price, int32(w.scale))
}

func (w *wantMarketBase) Fill(counter, price decimal.Decimal) {
	base := counter.Mul(price)
	w.remaining = w.remaining.Sub(base)
}

func (w *wantMarketBase) Filled() {
	w.remaining = decimal.Zero
}

func (w *wantMarketBase) IsFilled() bool {
	return w.remaining.Sign() == 0
}

type wantMarketCounter struct {
	remaining decimal.Decimal // Counter
}

func (w *wantMarketCounter) PriceLimit() decimal.Decimal {
	return decimal.Zero
}

func (w *wantMarketCounter) Remaining(_ decimal.Decimal) (counter decimal.Decimal) {
	return w.remaining
}

func (w *wantMarketCounter) Fill(counter, price decimal.Decimal) {
	w.remaining = w.remaining.Sub(counter)
}

func (w *wantMarketCounter) Filled() {
	w.remaining = decimal.Zero
}

func (w *wantMarketCounter) IsFilled() bool {
	return w.remaining.Sign() == 0
}

type wantLimit struct {
	price     decimal.Decimal
	remaining decimal.Decimal // Counter
}

func (w *wantLimit) PriceLimit() decimal.Decimal {
	return w.price
}

func (w *wantLimit) Remaining(_ decimal.Decimal) (counter decimal.Decimal) {
	return w.remaining
}

func (w *wantLimit) Fill(counter, price decimal.Decimal) {
	w.remaining = w.remaining.Sub(counter)
}

func (w *wantLimit) Filled() {
	w.remaining = decimal.Zero
}

func (w *wantLimit) IsFilled() bool {
	return w.remaining.Sign() == 0
}
