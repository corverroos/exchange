package matcher

import (
	"context"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

// Match applies commands to the order book and outputs
// results including trades. The order book and input commands
// should be sequential. The snap function allows taking
// snapshots of the order book. The latency function allows
// measuring match latency.
func Match(ctx context.Context, book OrderBook,
	input <-chan Command, output chan<- Result,
	scale int, snap func(*OrderBook), latency func() func()) error {

	for {
		var cmd Command
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cmd = <-input:
		}

		if cmd.Sequence <= book.Sequence {
			// Ignore old commands
			output <- Result{
				Sequence: cmd.Sequence,
				OrderID:  cmd.OrderID,
				Type:     TypeCommandOld,
			}
			continue
		} else if cmd.Sequence > book.Sequence+1 {
			return errors.New("out of order command",
				j.MKV{"expect": book.Sequence + 1, "got": cmd.Sequence})
		}

		l := latency()
		typ, tl := match(&book, cmd, scale)
		l()

		book.Sequence = cmd.Sequence

		output <- Result{
			Sequence: cmd.Sequence,
			OrderID:  cmd.OrderID,
			Type:     typ,
			Trades:   tl,
		}

		// Call some metrics
		snap(&book)
	}
}
