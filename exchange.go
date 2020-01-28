package exchange

import (
	"context"
	"database/sql"
	"exchange/db/cursors"
	"exchange/db/orders"
	"exchange/db/trades"
	"exchange/matcher"
	"strconv"
	"sync"

	"github.com/luno/jettison/j"
	"github.com/luno/reflex"

	"github.com/luno/fate"
	"github.com/luno/reflex/rpatterns"

	"github.com/luno/jettison/errors"
)

// Run runs the exchange returning the first error.
func Run(ctx context.Context, dbc *sql.DB) error {
	s := &state{
		dbc:    dbc,
		input:  make(chan matcher.Command, 1000),
		output: make(chan matcher.Result, 1000),
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
		// State processes results outputs trades, updates orders and acks events.
		// Process errors restart matching since result was removed from output channel.
		return s.ProcessResults(ctx)
	}):
	case err = <-goChan(func() error {
		// Match errors indicate bigger problems.
		return matcher.Match(ctx, book, s.input, s.output, 8, func(*matcher.OrderBook) {})
	}):
	}

	return err
}

// state encapsulated the exchange matcher state.
type state struct {
	dbc    *sql.DB
	input  chan matcher.Command
	output chan matcher.Result

	mu      sync.Mutex
	acks    []*rpatterns.AckEvent
	lastAck int64
}

func (s *state) ProcessResults(ctx context.Context) error {
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

	for r := range s.output {
		if r.Type == matcher.TypeCommandUnknown {
			// Ignore noops
			continue
		}

		s.mu.Lock()
		e := s.acks[0]
		s.acks = s.acks[1:]
		s.mu.Unlock()

		seq := r.Command.Sequence

		if e.IDInt() != seq {
			return errors.New("result ack not found",
				j.MKV{"want": seq, "got": e.IDInt()})
		}

		var completed []int64

		for i, t := range r.Trades {
			_, err := trades.Create(ctx, s.dbc, trades.CreateReq{
				IsBuy:        t.IsBuy,
				Seq:          seq,
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
			err := orders.UpdatePosted(ctx, s.dbc, r.Command.OrderID, seq)
			// TODO(corver): Ignore already posted errors
			if err != nil {
				return err
			}
		}

		if complete[r.Type] {
			completed = append(completed, r.Command.OrderID)
		}

		for _, id := range completed {
			err := orders.Complete(ctx, s.dbc, id, seq)
			// TODO(corver): Ignore already complete errors
			if err != nil {
				return err
			}
		}

		err := e.Ack(ctx)
		if err != nil {
			return err
		}
	}

	return errors.New("output channel closed")
}

func (s *state) Enqueue(ctx context.Context, fate fate.Fate, e *rpatterns.AckEvent) error {
	if !reflex.IsAnyType(e.Type, orders.StatusPending, orders.StatusCancelling) {
		// We only care about pending and cancelling states.
		return nil
	}

	o, err := orders.Lookup(ctx, s.dbc, e.ForeignIDInt())
	if err != nil {
		return err
	}

	var typ matcher.CommandType
	if reflex.IsType(e.Type, orders.StatusCancelling) {
		typ = matcher.CommandCancel
	} else if o.Type == orders.TypeMarket {
		typ = matcher.CommandMarket
	} else if o.Type == orders.TypePostOnly {
		typ = matcher.CommandPostOnly
	} else if o.Type == orders.TypeLimit {
		typ = matcher.CommandLimit
	} else {
		return errors.New("unsupported order type/status", j.KV("id", o.ID))
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

	cmd := matcher.Command{
		Sequence:      seq,
		Type:          typ,
		IsBuy:         o.IsBuy,
		OrderID:       o.ID,
		LimitPrice:    o.LimitPrice,
		LimitVolume:   o.LimitVolume,
		MarketBase:    o.MarketBase,
		MarketCounter: o.MarketCounter,
	}
	s.input <- cmd

	return nil
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
