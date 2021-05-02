package results

import (
	"encoding/json"
	"github.com/corverroos/exchange/matcher"
)

//go:generate glean -table=results

type glean struct {
	Result

	Results []byte `glean:"results_json"`
}

func (g glean) toResults() ([]matcher.Result, error) {
	var rl []matcher.Result
	err := json.Unmarshal(g.Results, &rl)
	if err != nil {
		return nil, err
	}

	return rl, nil
}
