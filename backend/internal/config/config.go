package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
)

type Config struct {
	HTTPPort    string
	DatabaseURL string
}

func Load(getenv func(string) string) (Config, error) {
	port := value(getenv, "HTTP_PORT", "8080")
	if !validPort(port) {
		return Config{}, errors.New("HTTP_PORT must be a port between 1 and 65535")
	}
	dbPort := value(getenv, "DB_PORT", "5432")
	if !validPort(dbPort) {
		return Config{}, errors.New("DB_PORT must be a port between 1 and 65535")
	}
	password := getenv("DB_PASSWORD")
	if password == "" {
		return Config{}, errors.New("DB_PASSWORD is required")
	}
	connection := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(value(getenv, "DB_USER", "crowfitness"), password),
		Host:     net.JoinHostPort(value(getenv, "DB_HOST", "127.0.0.1"), dbPort),
		Path:     "/" + value(getenv, "DB_NAME", "crowfitness"),
		RawQuery: "sslmode=disable&connect_timeout=2",
	}
	return Config{HTTPPort: port, DatabaseURL: connection.String()}, nil
}

func value(getenv func(string) string, name, fallback string) string {
	if result := getenv(name); result != "" {
		return result
	}
	return fallback
}

func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port > 0 && port <= 65535
}
