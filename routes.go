package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/labstack/echo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"FinPay/config"
	custhttp "FinPay/customer"
)

func newRouter(db *sql.DB) http.Handler {
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to initialize gorm db: %v", err)
	}

	e := echo.New()

	customerHandlers := custhttp.NewHandlers(gormDB, config.CoreConfig{})

	api := e.Group("/api")

	customerHandlers.CreateAuthenticatedEndpoints(api)

	return e
}
