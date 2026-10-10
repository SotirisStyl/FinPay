package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"FinPay/config"
	custhttp "FinPay/customer/http"
)

func newRouter(db *sql.DB) http.Handler {
	gormLogger := logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold:        time.Second,
		LogLevel:             logger.Info,
		ParameterizedQueries: true,
		Colorful:             false,
	})
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: gormLogger})
	if err != nil {
		log.Fatalf("failed to initialize gorm db: %v", err)
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	customerHandlers := custhttp.NewHandlers(gormDB, config.CoreConfig{})

	api := e.Group("/api")

	customerHandlers.CreateAuthenticatedEndpoints(api)

	return e
}
