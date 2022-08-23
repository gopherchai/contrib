package mysql

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	pkgerr "github.com/pkg/errors"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	return db, pkgerr.Wrapf(err, "open dsn:%s meet error", dsn)
}

type DB struct {
	write *sql.DB
	read  []*sql.DB
}

type breaker interface {
	Allow() bool
}
