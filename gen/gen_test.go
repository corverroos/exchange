package gen

import (
	"math/rand"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestGenRands(t *testing.T) {
	tests := []struct {
		Name string
		Req  Request
	}{
		{
			Name: "basic",
			Req: Request{
				Count:       10,
				Amount:      10,
				Price:       50,
				PriceStdDev: 5,
				PriceScale:  1,
				AmountScale: 2,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			test.Req.Rand = rand.New(rand.NewSource(0))

			ch := make(chan rands)
			go genRands(test.Req, ch)

			var rl []rands
			for r := range ch {
				rl = append(rl, r)
			}

			b, err := yaml.Marshal(rl)
			require.NoError(t, err)

			goldie.New(t).Assert(t, test.Name, b)
		})
	}

}
