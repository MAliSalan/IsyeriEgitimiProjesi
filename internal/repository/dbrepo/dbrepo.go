package dbrepo

import (
	"database/sql"

	"github.com/malisalan/sideproject/internal/config"
	"github.com/malisalan/sideproject/internal/repository"
)

type mysqlDBRepo struct {
	App *config.AppConfig
	DB  *sql.DB
}
type testDBRepo struct {
	App *config.AppConfig
	DB  *sql.DB
}

func NewMySQLRepo(conn *sql.DB, a *config.AppConfig) repository.DatabaseRepo {
	return &mysqlDBRepo{
		App: a,
		DB:  conn,
	}
}

func NewTestingRepo(a *config.AppConfig) repository.DatabaseRepo {
	return &testDBRepo{
		App: a,
	}
}
