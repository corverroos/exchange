package cursors

import (
	"database/sql"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

var cursors = rsql.NewCursorsTable("cursors")

func ToStore(dbc *sql.DB) reflex.CursorStore {
	return cursors.ToStore(dbc)
}
