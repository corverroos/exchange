package exchange

import (
	"sync/atomic"
)


type Metrics struct {
	getInput func() int
	getOutput func() int
	count int64 // Used with amotic
}

func (m *Metrics) InputLen() int{
	return m.getInput()
}
func (m *Metrics) OutputLen() int{
	return m.getOutput()
}
func (m *Metrics) Count() int64{
	return atomic.LoadInt64(&m.count)
}
func (m *Metrics) incCount() {
	atomic.AddInt64(&m.count, 1)
}
