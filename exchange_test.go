package exchange

import (
	"context"
	"database/sql"
	"exchange/db"
	"exchange/db/orders"
	"exchange/db/results"
	"exchange/gen"
	"exchange/matcher"
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/corverroos/unsure"
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/jtest"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var perfCount = flag.Int("perf_count", 10000, "performance test count")

func TestRun(t *testing.T) {
	defer unsure.CheatFateForTesting(t)()
	dbc := setupDB(t)
	ctx := context.Background()

	posts := 10
	// Create some post only orders
	for i := 0; i < posts; i++ {
		_, err := orders.CreateLimit(ctx, dbc, true, d(99-i), d(1), true)
		jtest.Require(t, nil, err)

		_, err = orders.CreateLimit(ctx, dbc, false, d(100+i), d(1), true)
		jtest.Require(t, nil, err)
	}

	// Cancel the first order
	err := orders.RequestCancel(ctx, dbc, 1)
	jtest.Require(t, nil, err)

	// Create some market orders
	markets := 5
	for i := 0; i < markets; i++ {
		_, err := orders.CreateMarketBuy(ctx, dbc, d(100))
		jtest.Require(t, nil, err)

		_, err = orders.CreateMarketSell(ctx, dbc, d(1))
		jtest.Require(t, nil, err)
	}

	// Cancel the first order
	err = orders.RequestCancel(ctx, dbc, 2)
	jtest.Require(t, nil, err)

	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err := Run(ctx2, dbc)
		jtest.Assert(t, context.Canceled, err)
	}()

	total := 2*posts + 2*markets + 2

	// Wait for all results
	waitFor(t, time.Second, func() bool {
		r, err := results.LookupLast(ctx, dbc)
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		assert.NoError(t, err)
		if r.EndSeq < int64(total) {
			return false
		}
		cancel()
		return true
	})

	// Check results
	rl, err := results.ListAll(ctx, dbc)
	require.NoError(t, err)

	var count int
	for _, result := range rl {
		for _, r := range result.Results {
			if int(r.Sequence) <= 2*posts {
				require.Equal(t, matcher.TypePosted, r.Type)
			} else if int(r.Sequence) == 2*posts+1 {
				require.Equal(t, matcher.TypeCancelled, r.Type)
			} else if int(r.Sequence) <= 2*posts+2*markets+1 {
				require.Equal(t, matcher.TypeMarketFull, r.Type)
			} else if int(r.Sequence) == 2*posts+2*markets+2 {
				require.Equal(t, matcher.TypeCancelFailed, r.Type)
			}
			count++
		}
	}

	require.Equal(t, total, count)

	// Consume results
	ctx3, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err := ConsumeResults(ctx3, dbc)
		jtest.Assert(t, context.Canceled, err)
	}()

	// Wait for orders to complete
	waitFor(t, time.Second, func() bool {
		o, err := orders.LookupLast(ctx, dbc)
		assert.NoError(t, err)
		if o.Status != orders.StatusComplete {
			return false
		}
		cancel()
		return true
	})

	// Check orders
	count = 0
	fn := func(o *orders.Order) error {
		switch o.Type {
		case orders.TypeMarket:
			require.Equal(t, orders.StatusComplete, o.Status)
		case orders.TypeLimit:
			if o.ID <= 10 {
				require.Equal(t, orders.StatusComplete, o.Status)
			} else {
				require.Equal(t, orders.StatusPosted, o.Status)
			}
		}
		count++
		return nil
	}

	err = orders.ScanAll(ctx, dbc, fn)
	jtest.Require(t, nil, err)
	require.Equal(t, 30, count)
}

func setupDB(t *testing.T) *sql.DB {
	err := flag.Lookup("db_recreate").Value.Set("true")
	require.NoError(t, err)

	dbc, err := db.Connect()
	require.NoError(t, err)

	return dbc
}

func waitFor(t *testing.T, timeout time.Duration, f func() bool) {
	t.Helper()
	t0 := time.Now()
	for {
		if f() {
			return
		}
		if time.Since(t0) < timeout {
			time.Sleep(time.Millisecond * 10) // Don't spin
			continue
		}
		t.Error("timeout waiting for")
		return
	}
}

func TestPerformance(t *testing.T) {
	count := float64(*perfCount)

	defer unsure.CheatFateForTesting(t)()
	dbc := setupDB(t)
	ctx := context.Background()
	r := rand.New(rand.NewSource(0))
	m := new(Metrics)
	d := new(depth)

	// Prep base request.
	req := gen.Request{
		Rand:         r,
		Amount:       0.1,
		AmountStdDev: 0.01,
		AmountScale:  4,
		Price:        100,
		PriceStdDev:  10,
		PriceScale:   2,
		CancelProb:   0.2,
	}

	// 5% sell post only
	postOnly := req
	postOnly.Count = int(count * 0.05)
	postOnly.Type = orders.TypePostOnly
	postOnly.Buy = false
	postOnly.Price = req.Price + 5
	err := gen.GenOrders(ctx, dbc, postOnly)
	require.NoError(t, err)

	// 5% buy post only
	postOnly.Buy = true
	postOnly.Price = req.Price - 5
	err = gen.GenOrders(ctx, dbc, postOnly)
	require.NoError(t, err)

	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Printf("Done with post only orders, starting exchange\n")
	t0 := time.Now()
	defer func() {
		fmt.Printf("Duration for %.0f orders: %s\n", count, time.Since(t0))
	}()

	// Start the exchange.
	go func() {
		err = Run(ctx2, dbc, WithMetrics(m), WithSnap(d.Set))
		jtest.Assert(t, context.Canceled, err)
		fmt.Printf("Done run\n")
	}()

	// Print metrics
	go func() {
		for ctx2.Err() == nil {
			time.Sleep(time.Second)
			printMetrics(d, m, t0)
		}
	}()

	var wg sync.WaitGroup

	// 35% limit sells
	wg.Add(1)
	go func() {
		limit := req
		limit.Rand = rand.New(rand.NewSource(0))
		limit.Count = int(count * 0.35)
		limit.Type = orders.TypeLimit
		limit.Buy = false
		limit.Price = req.Price + 1
		err := gen.GenOrders(ctx, dbc, limit)
		jtest.Assert(t, nil, err)
		wg.Done()
		fmt.Printf("Done limit sells: %v\n", limit.Count)
	}()

	// 35% limit buys
	wg.Add(1)
	go func() {
		limit := req
		limit.Rand = rand.New(rand.NewSource(0))
		limit.Count = int(count * 0.35)
		limit.Type = orders.TypeLimit
		limit.Buy = true
		limit.Price = req.Price + 1
		err := gen.GenOrders(ctx, dbc, limit)
		jtest.Assert(t, nil, err)
		wg.Done()
		fmt.Printf("Done limit buys: %v\n", limit.Count)
	}()

	// 10% market sells
	wg.Add(1)
	go func() {
		market := req
		market.Rand = rand.New(rand.NewSource(0))
		market.Count = int(count * 0.1)
		market.Type = orders.TypeMarket
		market.Buy = false
		err := gen.GenOrders(ctx, dbc, market)
		jtest.Assert(t, nil, err)
		wg.Done()
		fmt.Printf("Done market sells: %v\n", market.Count)
	}()

	// 10% market buys
	wg.Add(1)
	go func() {
		market := req
		market.Rand = rand.New(rand.NewSource(0))
		market.Count = int(count * 0.1)
		market.Type = orders.TypeMarket
		market.Buy = true
		err := gen.GenOrders(ctx, dbc, market)
		jtest.Assert(t, nil, err)
		wg.Done()
		fmt.Printf("Done market buys: %v\n", market.Count)
	}()

	wg.Wait()
	fmt.Printf("All orders created after: %v\n", time.Since(t0))

	// Create one last market order
	id, err := orders.CreateMarketSell(ctx, dbc, decimal.NewFromFloat(req.Amount))
	require.NoError(t, err)

	// Wait for last market order to be in the results.
	waitFor(t, time.Minute, func() bool {
		r, err := results.LookupLast(ctx, dbc)
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		assert.NoError(t, err)
		if len(r.Results) == 0 {
			t.Error(t, "result batch empty")
		}
		if r.Results[len(r.Results)-1].OrderID != id {
			return false
		}

		cancel()
		return true
	})
}

func d(i int) decimal.Decimal {
	return decimal.NewFromInt(int64(i))
}

func printMetrics(d *depth, m *Metrics, t0 time.Time) {
	c := m.Count()
	b, a := d.Get()
	fmt.Printf("Metrics: in=%d, out=%d, bids=%d, asks=%d, count=%d, latency=%s, rate=%0f cmds/s\n",
		m.InputLen(),
		m.OutputLen(),
		b,
		a,
		c,
		m.MeanLatency(),
		float64(c)/time.Since(t0).Seconds())
}

type depth struct {
	bids, asks int64
}

func (d *depth) Set(book *matcher.OrderBook) {
	atomic.StoreInt64(&d.bids, int64(len(book.Bids)))
	atomic.StoreInt64(&d.asks, int64(len(book.Asks)))
}
func (d *depth) Get() (int64, int64) {
	return atomic.LoadInt64(&d.bids), atomic.LoadInt64(&d.asks)
}
