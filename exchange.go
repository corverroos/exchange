package exchange

import (
	"context"
	"database/sql"
	"encoding/json"
	"exchange/db/cursors"
	"exchange/db/orders"
	"exchange/db/results"
	"exchange/db/trades"
	"exchange/matcher"
	"strconv"
	"sync"
	"time"

	"github.com/luno/fate"
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
)

// Run runs the exchange returning the first error.
func Run(ctx context.Context, dbc *sql.DB, opts ...Option) error {
	s := &state{
		dbc:       dbc,
		input:     make(chan matcher.Command, 1000),
		output:    make(chan matcher.Result, 1000),
		snap:      func(*matcher.OrderBook) {},
		baseScale: 8,
		countInc:  func() {},
		mLatency:  func() func() { return func() {} },
		maxBatch:  100,
	}

	for _, opt := range opts {
		opt(s)
	}

	cs := cursors.ToStore(dbc)
	name := "matcher"

	// Get current cursor (sequence)
	cursor, err := cs.GetCursor(ctx, name)
	if err != nil {
		return err
	}

	// Build order book up to sequence.
	var book matcher.OrderBook
	if cursor != "" {
		seq, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			return err
		}

		book, err = buildOrderBook(ctx, dbc, seq)
		if err != nil {
			return err
		}
	}

	// Reflex enqueues input from order events
	ac := rpatterns.NewAckConsumer(name, cs, s.Enqueue)
	spec := rpatterns.NewAckSpec(orders.ToStream(dbc), ac)

	// Start the input, matcher and output go routines, exit on first error.
	select {
	case err = <-goChan(func() error {
		// Reflex errors don't affect state, could actually just retry.
		return reflex.Run(ctx, spec)
	}):
	case err = <-goChan(func() error {
		// State stores results. Errors restart matching since
		// result is lost.
		return s.StoreResults(ctx)
	}):
	case err = <-goChan(func() error {
		// Match errors indicate bigger problems.
		return matcher.Match(ctx, book, s.input, s.output,
			s.baseScale, s.snap, s.mLatency)
	}):
	}

	return err
}

type Option func(*state)

func WithSnap(f func(book *matcher.OrderBook)) Option {
	return func(s *state) {
		s.snap = f
	}
}

func WithMetrics(m *Metrics) Option {
	return func(s *state) {
		s.countInc = m.incCount
		s.mLatency = m.latency
		m.getOutput = func() int {
			return len(s.output)
		}
		m.getInput = func() int {
			return len(s.input)
		}
	}
}

// state encapsulated the exchange matcher state.
type state struct {
	dbc    *sql.DB
	input  chan matcher.Command
	output chan matcher.Result
	snap   func(*matcher.OrderBook)

	mu      sync.Mutex
	acks    []*rpatterns.AckEvent
	lastAck int64

	baseScale int
	countInc  func()
	mLatency  func() func()
	maxBatch  int
}

func (s *state) StoreResults(ctx context.Context) error {
	for {
		// Read up to bax batch available results.
		var rl []matcher.Result
		for {
			// Pop a result if available on channel.
			var popped bool
			select {
			case r := <-s.output:
				rl = append(rl, r)
				popped = true
			default:
			}

			if !popped && len(rl) > 0 {
				// Nothing more available now, process batch.
				break
			} else if !popped && len(rl) == 0 {
				// Nothing available yet, wait a bit.
				time.Sleep(time.Millisecond)
				continue
			} else if popped && len(rl) >= s.maxBatch {
				// Max popped, process batch
				break
			} else /* popped && len(rl) < s.maxBatch */ {
				// Popped another, see if more available.
				continue
			}
		}

		var (
			toStore []matcher.Result
			toAck   *rpatterns.AckEvent
		)
		for _, r := range rl {
			if r.Type == matcher.TypeCommandUnknown {
				// Ignore noops
				continue
			}
			s.countInc() // Do not include noops in "count" metrics.

			s.mu.Lock()
			e := s.acks[0]
			s.acks = s.acks[1:]
			s.mu.Unlock()

			seq := r.Sequence

			if e.IDInt() != seq {
				return errors.New("result ack not found",
					j.MKV{"want": seq, "got": e.IDInt()})
			}

			toAck = e
			toStore = append(toStore, r)
		}

		if len(toStore) == 0 {
			continue
		}

		_, err := results.Create(ctx, s.dbc, toStore)
		if err != nil {
			return err
		}

		err = toAck.Ack(ctx)
		if err != nil {
			return err
		}
	}
}

func (s *state) Enqueue(ctx context.Context, fate fate.Fate, e *rpatterns.AckEvent) error {
	var (
		cmd matcher.Command
		err error
	)
	if reflex.IsType(e.Type, orders.StatusPending) {
		cmd, err = makeCreate(e)
	} else if reflex.IsType(e.Type, orders.StatusCancelling) {
		cmd, err = makeCancel(e)
	} else {
		// We only care about pending and cancelling states.
		return nil
	}
	if err != nil {
		return err
	}

	seq := e.IDInt()

	s.mu.Lock()
	if s.lastAck >= seq {
		// Event already enqueued.
		s.mu.Unlock()
		return nil
	}
	prevSeq := s.lastAck
	s.lastAck = seq
	s.acks = append(s.acks, e)
	s.mu.Unlock()

	// Reflex filters noops, but matcher requires sequential commands
	for i := prevSeq + 1; i < seq; i++ {
		s.input <- matcher.Command{Sequence: i}
	}

	s.input <- cmd

	return nil
}

func makeCancel(e *rpatterns.AckEvent) (matcher.Command, error) {
	var isBuy bool
	err := json.Unmarshal(e.MetaData, &isBuy)
	if err != nil {
		return matcher.Command{}, err
	}

	return matcher.Command{
		Sequence: e.IDInt(),
		Type:     matcher.CommandCancel,
		IsBuy:    isBuy,
		OrderID:  e.ForeignIDInt(),
	}, nil
}

func makeCreate(e *rpatterns.AckEvent) (matcher.Command, error) {
	var req orders.CreateReq
	err := json.Unmarshal(e.MetaData, &req)
	if err != nil {
		return matcher.Command{}, err
	}

	var typ matcher.CommandType
	if req.Type == orders.TypeMarket {
		typ = matcher.CommandMarket
	} else if req.Type == orders.TypePostOnly {
		typ = matcher.CommandPostOnly
	} else if req.Type == orders.TypeLimit {
		typ = matcher.CommandLimit
	} else {
		return matcher.Command{}, errors.New("unsupported order type/status",
			j.KV("id", e.ForeignIDInt()))
	}

	return matcher.Command{
		Sequence:      e.IDInt(),
		Type:          typ,
		IsBuy:         req.IsBuy,
		OrderID:       e.ForeignIDInt(),
		LimitPrice:    req.LimitPrice,
		LimitVolume:   req.LimitVolume,
		MarketBase:    req.MarketBase,
		MarketCounter: req.MarketCounter,
	}, nil
}

func buildOrderBook(ctx context.Context, db *sql.DB, seq int64) (matcher.OrderBook, error) {
	return matcher.OrderBook{}, errors.New("buildOrderBook not implemented")
}

func goChan(f func() error) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- f()
		close(ch)
	}()
	return ch
}

func ConsumeResults(ctx context.Context, dbc *sql.DB) error {
	spec := reflex.NewSpec(
		results.ToStream(dbc),
		cursors.ToStore(dbc),
		makeResultConsumec(dbc),
	)
	return reflex.Run(ctx, spec)
}

func makeResultConsumec(dbc *sql.DB) reflex.Consumer {
	// These results always complete orders.
	complete := map[matcher.Type]bool{
		matcher.TypeLimitTaker:    true,
		matcher.TypeMarketEmpty:   true,
		matcher.TypeMarketPartial: true,
		matcher.TypeMarketFull:    true,
		matcher.TypeCancelled:     true,
	}

	posted := map[matcher.Type]bool{
		matcher.TypePosted:       true,
		matcher.TypeLimitMaker:   true,
		matcher.TypeLimitPartial: true,
	}

	return reflex.NewConsumer("result_consumer",
		func(ctx context.Context, f fate.Fate, e *reflex.Event) error {

			result, err := results.Lookup(ctx, dbc, e.ForeignIDInt())
			if err != nil {
				return err
			}

			for _, r := range result.Results {

				var completed []int64

				for i, t := range r.Trades {
					_, err := trades.Create(ctx, dbc, trades.CreateReq{
						IsBuy:        t.IsBuy,
						Seq:          r.Sequence,
						SeqIdx:       i,
						Price:        t.Price,
						Volume:       t.Volume,
						MakerOrderID: t.MakerOrderID,
						TakerOrderID: t.TakerOrderID,
					})
					// TODO(corver): Ignore duplicate on uniq index
					if err != nil {
						return err
					}

					if t.MakerFilled {
						completed = append(completed, t.MakerOrderID)
					}
				}

				if posted[r.Type] {
					err := orders.UpdatePosted(ctx, dbc, r.OrderID, r.Sequence)
					// TODO(corver): Ignore already posted errors
					if err != nil {
						return err
					}
				}

				if complete[r.Type] {
					completed = append(completed, r.OrderID)
				}

				for _, id := range completed {
					err := orders.Complete(ctx, dbc, id, r.Sequence)
					// TODO(corver): Ignore already complete errors
					if err != nil {
						return err
					}
				}
			}

			return nil
		},
	)
}
