package config

import (
	"net/url"
	"strings"
	"testing"
)

func TestLoadEncodesCredentials(t *testing.T) {
	values := map[string]string{"DB_PASSWORD": "pass:@/word?#", "DB_HOST": "postgres"}
	cfg, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if password != values["DB_PASSWORD"] || parsed.Host != "postgres:5432" || cfg.HTTPPort != "8080" {
		t.Fatal("configuration did not preserve credentials or defaults")
	}
}

func TestLoadRejectsInvalidConfigurationWithoutLeakingPassword(t *testing.T) {
	for _, values := range []map[string]string{
		{},
		{"DB_PASSWORD": "private", "HTTP_PORT": "0"},
		{"DB_PASSWORD": "private", "DB_PORT": "65536"},
		{"DB_PASSWORD": "private", "HTTP_PORT": "abc"},
	} {
		_, err := Load(func(key string) string { return values[key] })
		if err == nil {
			t.Fatal("expected invalid configuration to fail")
		}
		if strings.Contains(err.Error(), "private") {
			t.Fatal("error leaked the password")
		}
	}
}

func TestExplicitDevResetSetting(t *testing.T) {
	for _, tc := range []struct {
		value            string
		enabled, invalid bool
	}{{"", false, false}, {"false", false, false}, {"true", true, false}, {"yes", false, true}} {
		cfg, err := Load(func(key string) string {
			if key == "DB_PASSWORD" {
				return "disposable"
			}
			if key == "ENABLE_DEV_RESET" {
				return tc.value
			}
			return ""
		})
		if (err != nil) != tc.invalid || cfg.EnableDevReset != tc.enabled {
			t.Fatal(tc, cfg.EnableDevReset, err)
		}
	}
}
