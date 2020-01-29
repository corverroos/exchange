package results

import (
	"database/sql"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

var (
	events = rsql.NewEventsTableInt("result_events",
		rsql.WithEventsInMemNotifier())
)

func ToStream(dbc *sql.DB) reflex.StreamFunc {
	return events.ToStream(dbc)
}
