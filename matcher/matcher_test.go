package matcher

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/jtest"
	"github.com/sebdah/goldie/v2"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestBasic(t *testing.T) {
	cmds := []Command{
		{
			// CommandOld
		},
		{
			// CommandUnknown
			Type: CommandUnknown,
		},
		{
			// MarketEmpty sell
			Type:          CommandMarket,
			MarketCounter: d(1),
			IsBuy:         false,
		},
		{
			// MarketEmpty buy
			Type:       CommandMarket,
			MarketBase: d(1),
			IsBuy:      true,
		},
		{
			// LimitMaker Ask:1@12
			Type:        CommandLimit,
			LimitPrice:  d(12),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitMaker Bid:1@8
			Type:        CommandLimit,
			LimitPrice:  d(8),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// Posted Ask:1@11
			Type:        CommandPostOnly,
			LimitPrice:  d(11),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// PostFailed (on previous)
			Type:        CommandPostOnly,
			LimitPrice:  d(11),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// Posted Bid:1@9
			Type:        CommandPostOnly,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// PostFailed (on previous)
			Type:        CommandPostOnly,
			LimitPrice:  d(5),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitTaker Bid:1@9
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitTaker Ask:1@11
			Type:        CommandLimit,
			LimitPrice:  d(11),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitPartial: Take Bid:1@8 + Make Ask:1@8
			Type:        CommandLimit,
			LimitPrice:  d(8),
			LimitVolume: d(2),
			IsBuy:       false,
		},
		{
			// LimitPartial Take Ask:1@8 + Make Bid:1@8
			Type:        CommandLimit,
			LimitPrice:  d(8),
			LimitVolume: d(2),
			IsBuy:       true,
		},
	}
	testMatch(t, cmds)
}

func TestMarketCounter(t *testing.T) {
	cmds := []Command{{ /* CommandOld*/ },
		{
			// LimitMaker Bid:1@10
			Type:        CommandLimit,
			LimitPrice:  d(10),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@9
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@8
			Type:        CommandLimit,
			LimitPrice:  d(8),
			LimitVolume: d(2),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@8
			Type:          CommandMarket,
			MarketCounter: d(3),
			IsBuy:         false,
		},
	}
	testMatch(t, cmds)
}

func TestMarketBase1(t *testing.T) {
	cmds := []Command{{ /* CommandOld*/ },
		{
			// LimitMaker Ask:1@10
			Type:        CommandLimit,
			LimitPrice:  d(10),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitMaker Ask:1@12
			Type:        CommandLimit,
			LimitPrice:  d(12),
			LimitVolume: d(2),
			IsBuy:       false,
		},
		{
			// LimitMaker Ask:1@8
			Type:        CommandLimit,
			LimitPrice:  d(11),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitMakerFull
			Type:       CommandMarket,
			MarketBase: d(10*1 + 11*1 + 12*1),
			IsBuy:      true,
		},
	}
	testMatch(t, cmds)
}

func TestCancel(t *testing.T) {
	cmds := []Command{{ /* CommandOld*/ },
		{
			// LimitMaker Ask:1@10
			Type:        CommandLimit,
			LimitPrice:  d(10),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitMaker Ask:1@12
			Type:        CommandLimit,
			LimitPrice:  d(12),
			LimitVolume: d(2),
			IsBuy:       false,
		},
		{
			// LimitCancelFailed
			Type:    CommandCancel,
			OrderID: 100,
			IsBuy:   false,
		},
		{
			// LimitCancel
			Type:    CommandCancel,
			OrderID: 2,
			IsBuy:   false,
		},
		{
			// LimitMakerFull
			Type:       CommandMarket,
			MarketBase: d(5 * 1),
			IsBuy:      true,
		},
		{
			// LimitCancel
			Type:    CommandCancel,
			OrderID: 1,
			IsBuy:   false,
		},
		{
			// LimitCancel
			Type:    CommandCancel,
			OrderID: 1,
			IsBuy:   false,
		},
	}
	testMatch(t, cmds)
}

func TestLimitTaker1(t *testing.T) {
	cmds := []Command{{ /* CommandOld*/ },
		{
			// LimitMaker Bid:1@10
			Type:        CommandLimit,
			LimitPrice:  d(10),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@9
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@9
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@8
			Type:        CommandLimit,
			LimitPrice:  d(8),
			LimitVolume: d(2),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@8
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(2),
			IsBuy:       false,
		},
	}
	testMatch(t, cmds)
}

func TestLimitPartial(t *testing.T) {
	cmds := []Command{{ /* CommandOld*/ },
		{
			// LimitMaker Bid:1@10
			Type:        CommandLimit,
			LimitPrice:  d(10),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@9
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@9
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(1),
			IsBuy:       true,
		},
		{
			// LimitMaker Bid:1@8
			Type:        CommandLimit,
			LimitPrice:  d(8),
			LimitVolume: d(2),
			IsBuy:       true,
		},
		{
			// LimitPartial
			Type:        CommandLimit,
			LimitPrice:  d(9),
			LimitVolume: d(5),
			IsBuy:       false,
		},
	}
	testMatch(t, cmds)
}

func TestMarketBase2(t *testing.T) {
	cmds := []Command{{ /* CommandOld*/ },
		{
			// LimitMaker Ask:1@10
			Type:        CommandLimit,
			LimitPrice:  d(10),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitMaker Ask:1@12
			Type:        CommandLimit,
			LimitPrice:  d(12),
			LimitVolume: d(2),
			IsBuy:       false,
		},
		{
			// LimitMaker Ask:1@8
			Type:        CommandLimit,
			LimitPrice:  d(11),
			LimitVolume: d(1),
			IsBuy:       false,
		},
		{
			// LimitMakerFull
			Type:       CommandMarket,
			MarketBase: d(3 * 10),
			IsBuy:      true,
		},
	}
	testMatch(t, cmds)
}

func testMatch(t *testing.T, cmds []Command) {

	count := len(cmds)
	ctx := &ctx{count: count}
	input := make(chan Command, count)
	output := make(chan Result, count)

	for i, cmd := range cmds {
		cmd.Sequence = int64(i)

		// Auto fill non-cancel order ids.
		if cmd.Type != CommandCancel {
			cmd.OrderID = int64(i)
		}
		input <- cmd
	}

	books := make(map[int64]string)
	latency := func() func() { return func() {} }
	snap := func(book *OrderBook) {
		books[book.Sequence] = printBook(book)
	}

	err := Match(ctx, OrderBook{}, input, output, 8, snap, latency)
	jtest.Require(t, ctxDone, err)
	require.Len(t, output, count)

	type r struct {
		Seq    int64
		Type   string
		Trades []Trade
		Book   string
	}
	var rl []r
	close(output)
	for i := 0; i < count; i++ {
		o := <-output
		seq := o.Sequence
		require.Equal(t, int64(i), seq)
		rl = append(rl, r{
			Seq:    seq,
			Type:   o.Type.String(),
			Trades: o.Trades,
			Book:   books[seq] + "\n\n",
		})
	}

	y, err := yaml.Marshal(rl)
	goldie.New(t).Assert(t, t.Name(), y)
}

func d(i int64) decimal.Decimal {
	return decimal.NewFromInt(i)
}

var ctxDone = errors.New("done", j.C("ERR_1b0980445da57da5"))

type ctx struct {
	context.Context
	count int
}

func (c *ctx) Done() <-chan struct{} {
	c.count--
	ch := make(chan struct{})
	if c.count < 0 {
		close(ch)
	}
	return ch
}

func (c *ctx) Err() error {
	return ctxDone
}

func printBook(book *OrderBook) string {
	var sb strings.Builder

	asks := reserve(printSide(book.Asks))
	sb.WriteString(strings.Join(asks, "\n"))
	sb.WriteString("\n-------\n")
	bids := printSide(book.Bids)
	sb.WriteString(strings.Join(bids, "\n"))
	sb.WriteString("\n")

	return sb.String()
}

func reserve(sl []string) []string {
	var res []string
	for i := len(sl) - 1; i >= 0; i-- {
		res = append(res, sl[i])
	}
	return res
}

func printSide(side []Order) []string {
	var res []string
	var line string
	for _, o := range side {
		if strings.Contains(line, o.Price.String()) {
			line += ", " + o.Remaining.String()
		} else {
			if line != "" {
				res = append(res, line)
			}
			line = fmt.Sprintf("%s: %s", o.Price, o.Remaining)
		}
	}

	if line == "" {
		line = "empty"
	}
	res = append(res, line)

	return res
}
