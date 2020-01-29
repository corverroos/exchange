package results

import (
	"encoding/json"
	"exchange/matcher"
)

//go:generate glean -table=results

type glean struct {
	Result

	Trades []byte `glean:"trades_json"`
}

func (g glean) toTrades() ([]matcher.Trade, error) {
	var tl []matcher.Trade
	err := json.Unmarshal(g.Trades, &tl)
	if err != nil {
		return nil, err
	}

	return tl, nil
}
