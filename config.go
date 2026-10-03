package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
)

func buildDSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(getEnv("DB_USER", "postgres"), mustGetEnv("DB_PASSWORD")),
		Host:     fmt.Sprintf("%s:%s", getEnv("DB_HOST", "localhost"), getEnv("DB_PORT", "5432")),
		Path:     getEnv("DB_NAME", "finpay_db"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}
