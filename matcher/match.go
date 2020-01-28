package matcher

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// match applies the command to the order book and returns
// the match type and any trades.
func match(book *OrderBook, cmd Command, scale int) (Type, []Trade) {
	switch cmd.Type {

	case CommandUnknown:
		return TypeCommandUnknown, nil

	case CommandCancel:
		ok := cancelOrder(book, cmd)
		if !ok {
			return TypeCancelFailed, nil
		}
		return TypeCancelled, nil

	case CommandPostOnly:
		ok := postLimit(book, cmd, cmd.LimitVolume)
		if !ok {
			return TypePostFailed, nil
		}
		return TypePosted, nil

	case CommandMarket:
		tl, ok := applyMarket(book, cmd, scale)
		var typ Type
		if !ok && len(tl) == 0 {
			typ = TypeMarketEmpty
		} else if !ok {
			typ = TypeMarketPartial
		} else {
			typ = TypeMarketFull
		}
		return typ, tl

	case CommandLimit:
		tl, ok := applyLimit(book, cmd)
		var typ Type
		if ok && len(tl) == 0 {
			typ = TypeLimitMaker
		} else if ok {
			typ = TypeLimitPartial
		} else {
			typ = TypeLimitTaker
		}
		return typ, tl

	default:
		panic("unknonn command")
	}
}

// applyLimit applies the limit order to the orderbook and
// returns any trades and true if an order was inserted in the book.
func applyLimit(book *OrderBook, cmd Command) ([]Trade, bool) {
	w := &wantLimit{
		price:     cmd.LimitPrice,
		remaining: cmd.LimitVolume,
	}

	tl := trade(book, cmd, w)

	if w.IsFilled() {
		return tl, false
	}

	ok := postLimit(book, cmd, w.remaining)
	if !ok {
		panic(fmt.Sprintf("unexpected post failed: %d", cmd.Sequence))
	}

	return tl, true
}

// applyMarket applies the market order to the orderbook and
// returns any trades and true if the order was filled.
func applyMarket(book *OrderBook, cmd Command, scale int) ([]Trade, bool) {
	var want want
	if cmd.IsBuy {
		want = &wantMarketBase{remaining: cmd.MarketBase, scale: scale}
	} else {
		want = &wantMarketCounter{remaining: cmd.MarketCounter}
	}

	tl := trade(book, cmd, want)

	return tl, want.IsFilled()
}

// trade applies the want request to the order book and returns
// any trades.
func trade(book *OrderBook, cmd Command, want want) []Trade {
	var side []Order
	if cmd.IsBuy {
		// Buy orders match asks.
		side = book.Asks
	} else {
		// Sell orders match bids.
		side = book.Bids
	}

	var (
		trades []Trade
		pop    int // Number of orders to pop off (filled).
	)
	for i, o := range side {
		// If want limit is not enough, trades are done.
		if isInside(o, want.PriceLimit(), !cmd.IsBuy) {
			break
		}

		t := Trade{
			MakerOrderID: o.ID,
			TakerOrderID: cmd.OrderID,
			Price:        o.Price,
			IsBuy:        cmd.IsBuy,
		}

		wantRemaining := want.Remaining(o.Price)
		diff := wantRemaining.Sub(o.Remaining)

		if diff.Sign() < 0 {
			// Got all wanted (taker filled)
			// Filled partial order
			want.Filled()
			t.Volume = wantRemaining
			side[i].Remaining = diff.Abs()

		} else if diff.Sign() > 0 {
			// Got some wanted
			// Filled whole order (maker filled)
			want.Fill(o.Remaining, o.Price)
			t.Volume = o.Remaining
			t.MakerFilled = true
			pop++

		} else /* diff.Sign() == 0 */ {
			// Got all wanted (taker filled)
			// Filled whole order (maker filled)
			want.Filled()
			t.Volume = o.Remaining
			t.MakerFilled = true
			pop++
		}

		trades = append(trades, t)
		if want.IsFilled() {
			break
		}
	}

	if cmd.IsBuy {
		book.Asks = side[pop:]
	} else {
		book.Bids = side[pop:]
	}

	return trades
}

// postLimit adds the limit order to the book or returns false if
// it would result in a trade.
func postLimit(book *OrderBook, cmd Command, remaining decimal.Decimal) bool {

	if cmd.IsBuy {
		// Check if buy order matches lowest ask.
		if len(book.Asks) > 0 &&
			!isInside(book.Asks[0], cmd.LimitPrice, false) {
			return false
		}
	} else {
		// Check if sell order matches highest bid.
		if len(book.Bids) > 0 &&
			!isInside(book.Bids[0], cmd.LimitPrice, true) {
			return false
		}
	}

	var side []Order
	if cmd.IsBuy {
		// Buy limit orders are posted to bids.
		side = book.Bids
	} else {
		// Sell limit orders are posted to asks.
		side = book.Asks
	}

	// Find index to insert at.
	var idx int
	for _, o := range side {
		if isInside(o, cmd.LimitPrice, cmd.IsBuy) {
			break
		}
		idx++
	}

	o := Order{
		ID:        cmd.OrderID,
		Price:     cmd.LimitPrice,
		Remaining: remaining,
	}

	temp := append([]Order(nil), side[:idx]...)
	temp = append(temp, o)
	temp = append(temp, side[idx:]...)
	if cmd.IsBuy {
		book.Bids = temp
	} else {
		book.Asks = temp
	}
	return true
}

// cancelOrder returns true if the order was removed from the book.
func cancelOrder(book *OrderBook, cmd Command) bool {
	var side []Order
	if cmd.IsBuy {
		// Buy limit orders are posted to bids.
		side = book.Bids
	} else {
		// Sell limit orders are posted to asks.
		side = book.Asks
	}

	// Find the order to cancel.
	var idx int
	for _, o := range side {
		if o.ID == cmd.OrderID {
			break
		}
		idx++
	}

	// Not found
	if idx == len(side) {
		return false
	}

	// Remove from the book.
	temp := append(side[:idx], side[idx+1:]...)
	if cmd.IsBuy {
		book.Bids = temp
	} else {
		book.Asks = temp
	}

	return true
}

// isInside returns true if the price is "inside" the order.
// For bids the inside price is higher.
// For asks the inside price is lower.
// A zero price returns false.
func isInside(x Order, price decimal.Decimal, isBid bool) bool {
	if price.Sign() == 0 {
		return false
	}

	diff := x.Price.Cmp(price)
	if diff == 0 {
		// Equal, so not inside.
		return false
	}
	priceLower := diff > 0
	priceHigher := diff < 0
	return isBid && priceHigher || !isBid && priceLower
}
