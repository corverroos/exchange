// Package gen provides functionality for generating orders easily.
package gen

import (
	"context"
	"database/sql"
	"exchange/db/orders"
	"math/rand"

	"github.com/shopspring/decimal"
)

// Request defines an order generation request.
type Request struct {
	Rand  *rand.Rand  // Rand for deterministic behaviour
	Count int         // Number of order to create
	Type  orders.Type // Type of orders to create.
	Buy   bool        // Buys or sells

	Amount       float64 // Counter amount to buy/sell.
	AmountStdDev float64 // Standard deviation volume fuzz (10% of volume is good start)
	AmountScale  int     // Scale for amount/counter.

	Price       float64 // Price to aim at
	PriceStdDev float64 // Standard deviation price fuzz (10% of price is good start)
	PriceScale  int     // Scale for prices/base.

	CancelProb float64 // Probability limit order will be cancelled.
}

// GenOrders create random orders based on the request values.
func GenOrders(ctx context.Context, dbc *sql.DB, req Request) error {
	ch := make(chan rands, 1000)
	go genRands(req, ch)

	var cancels []int64

	for rands := range ch {
		if req.Type == orders.TypeMarket && req.Buy {
			_, err := orders.CreateMarketBuy(ctx, dbc, rands.MarketBase)
			if err != nil {
				return err
			}
		} else if req.Type == orders.TypeMarket && !req.Buy {
			_, err := orders.CreateMarketSell(ctx, dbc, rands.MarketCount)
			if err != nil {
				return err
			}
		} else {
			id, err := orders.CreateLimit(ctx, dbc, req.Buy, rands.LimitPrice,
				rands.LimitVolume, req.Type == orders.TypePostOnly)
			if err != nil {
				return err
			}

			// Maybe add to future cancels.
			if rands.Floats[0] < req.CancelProb {
				cancels = append(cancels, id)
			}
		}

		// Maybe cancel one previous
		if len(cancels) > 0 && rands.Floats[1] < req.CancelProb {
			// Pick either head or tail.
			var id int64
			if rands.Floats[2] < 0.5 {
				id = cancels[0]
				cancels = cancels[1:]
			} else {
				last := len(cancels) - 1
				id = cancels[last]
				cancels = cancels[:last]
			}

			err := orders.RequestCancel(ctx, dbc, id)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type rands struct {
	MarketBase  decimal.Decimal
	MarketCount decimal.Decimal
	LimitPrice  decimal.Decimal
	LimitVolume decimal.Decimal

	// Floats provide 5 random floats for custom logic.
	Floats [5]float64 `json:"omit"` // Omit this in test for brevity.
}

// genRands returns req.Count deterministic rands structs.
func genRands(req Request, ch chan<- rands) {
	for i := 0; i < req.Count; i++ {

		price, priceDec := fuzz(req.Rand, req.Price, req.PriceStdDev)
		vol, volDec := fuzz(req.Rand, req.Amount, req.AmountStdDev)

		base := vol * price
		baseDec := decimal.NewFromFloat(base)

		volDec = volDec.Round(int32(req.AmountScale))
		priceDec = priceDec.Round(int32(req.PriceScale))
		baseDec = baseDec.Round(int32(req.PriceScale))

		var floats [5]float64
		for i := 0; i < 5; i++ {
			floats[i] = req.Rand.Float64()
		}

		ch <- rands{
			MarketBase:  baseDec,
			MarketCount: volDec,
			LimitPrice:  priceDec,
			LimitVolume: volDec,
			Floats:      floats,
		}
	}

	close(ch)
}

func fuzz(r *rand.Rand, mean, stdDev float64) (float64, decimal.Decimal) {
	res := r.NormFloat64()*stdDev + mean
	return res, decimal.NewFromFloat(res)
}
