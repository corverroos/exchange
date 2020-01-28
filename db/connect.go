package db

import (
	"database/sql"
	"flag"
	"runtime"
	"strings"
	"testing"

	"github.com/corverroos/unsure"
	"github.com/luno/jettison/log"
)

var (
	dbURI = flag.String("db", "mysql://root@unix("+unsure.SockFile()+")/exchange?",
		"engine DB URI")
)

func Connect() (*sql.DB, error) {
	ok, err := unsure.MaybeRecreateSchema(*dbURI, getSchemaPath())
	if err != nil {
		return nil, err
	} else if ok {
		log.Info(nil, "recreated schema")
	}

	return unsure.Connect(*dbURI)
}

func ConnectForTesting(t *testing.T) *sql.DB {
	return unsure.ConnectForTesting(t, getSchemaPath())
}

func getSchemaPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return strings.Replace(filename, "connect.go", "schema.sql", 1)
}
