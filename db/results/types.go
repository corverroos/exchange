package results

import (
	"github.com/corverroos/exchange/matcher"
	"time"
)

type Result struct {
	ID        int64
	StartSeq  int64
	EndSeq    int64
	CreatedAt time.Time
	Results   []matcher.Result
}
