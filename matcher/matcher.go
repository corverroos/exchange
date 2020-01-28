package matcher

import (
	"context"

	"github.com/luno/jettison/j"

	"github.com/luno/jettison/errors"
)

// Match applies commands to the order book and outputs
// results including trades. The order book and input commands
// should be sequential. The snap function allows taking
// snapshots of the order book.
func Match(ctx context.Context, book OrderBook,
	input <-chan Command, output chan<- Result,
	scale int, snap func(*OrderBook)) error {

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
				Type:    TypeCommandOld,
				Command: cmd,
			}
			continue
		} else if cmd.Sequence > book.Sequence+1 {
			return errors.New("out of order command",
				j.MKV{"expect": book.Sequence + 1, "got": cmd.Sequence})
		}

		typ, tl := match(&book, cmd, scale)

		book.Sequence = cmd.Sequence

		output <- Result{
			Type:    typ,
			Trades:  tl,
			Command: cmd,
		}

		snap(&book)
	}
}
