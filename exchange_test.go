package exchange

import (
	"context"
	"exchange/db"
	"exchange/db/orders"
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/corverroos/unsure"
	"github.com/shopspring/decimal"

	"github.com/luno/jettison/jtest"
)

func TestRun(t *testing.T) {
	defer unsure.CheatFateForTesting(t)()

	err := flag.Lookup("db_recreate").Value.Set("true")
	require.NoError(t, err)

	dbc, err := db.Connect()
	require.NoError(t, err)

	ctx := context.Background()

	// Create some post only orders
	for i := 0; i < 10; i++ {
		_, err := orders.CreateLimit(ctx, dbc, true, d(100-i), d(1), true)
		jtest.Require(t, nil, err)

		_, err = orders.CreateLimit(ctx, dbc, false, d(100+i), d(1), true)
		jtest.Require(t, nil, err)
	}

	// Cancel the first order
	err = orders.RequestCancel(ctx, dbc, 1)
	jtest.Require(t, nil, err)

	// Create some market orders
	for i := 0; i < 5; i++ {
		_, err := orders.CreateMarketBuy(ctx, dbc, d(100))
		jtest.Require(t, nil, err)

		_, err = orders.CreateMarketSell(ctx, dbc, d(1))
		jtest.Require(t, nil, err)
	}

	err = orders.RequestCancel(ctx, dbc, 2)
	jtest.Require(t, nil, err)

	ctx2, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	err = Run(ctx2, dbc)
	jtest.Require(t, context.DeadlineExceeded, err)

	count := 0
	fn := func(o *orders.Order) error {
		count++
		return nil
	}

	err = orders.ScanAll(ctx, dbc, fn)
	jtest.Require(t, nil, err)
}

func d(i int) decimal.Decimal {
	return decimal.NewFromInt(int64(i))
}
