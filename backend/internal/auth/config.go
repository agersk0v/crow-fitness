package auth

import (
	"errors"
	"net/url"
	"strings"
)

type Config struct {
	Origin, ClientID, ClientSecret string
	Production                     bool
}

func ConfigFromEnv(getenv func(string) string) (Config, error) {
	c := Config{Origin: strings.TrimRight(getenv("PUBLIC_ORIGIN"), "/"), ClientID: getenv("GOOGLE_CLIENT_ID"), ClientSecret: getenv("GOOGLE_CLIENT_SECRET"), Production: getenv("APP_ENV") == "production"}
	if getenv("APP_ENV") != "production" && getenv("APP_ENV") != "development" {
		return c, errors.New("APP_ENV must be production or development")
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return c, errors.New("PUBLIC_ORIGIN must be an exact HTTP(S) origin")
	}
	if c.Production && (u.Scheme != "https" || getenv("ENABLE_DEV_RESET") == "true") {
		return c, errors.New("production requires HTTPS and disabled dev reset")
	}
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return c, errors.New("HTTP is allowed only on a loopback development origin")
	}
	if c.ClientID == "" || c.ClientSecret == "" {
		return c, errors.New("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required")
	}
	return c, nil
}
