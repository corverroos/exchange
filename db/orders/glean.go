package orders

import "database/sql"

//go:generate glean -table=orders -scan

type glean struct {
	Order

	UpdateSeq sql.NullInt64
}
