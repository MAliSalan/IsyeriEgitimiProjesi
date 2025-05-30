package driver

import (
	"database/sql"
	"time"
)

type DB struct {
	SQL *sql.DB
}

var dbConn = &DB{}

const maxOpenDbConn = 10
const maxIdleDbConn = 5
const maxDbLifetime = 5 * time.Minute

func ConnectSQL(dsn string) (*DB, error) {
	d, err := NewDatabase(dsn)
	if err != nil {
		panic(err)
	}
	d.SQL.SetMaxOpenConns(maxOpenDbConn)
	d.SQL.SetMaxIdleConns(maxIdleDbConn)
	d.SQL.SetConnMaxLifetime(maxDbLifetime)
	dbConn.SQL = d.SQL
	err = testDB(d.SQL)
	if err != nil {
		return nil, err
	}
	return dbConn, nil
}

func testDB(d *sql.DB) error {
	err := d.Ping()
	if err != nil {
		return err
	}
	return nil
}

func NewDatabase(dsn string) (*DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DB{SQL: db}, nil
}
