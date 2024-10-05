package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Validate(t *testing.T) {
	type test struct {
		name  string
		conf  *Config
		isErr bool
	}

	newValidConfig := func() *Config {
		conf := newDefaultConfig()

		conf.Bot.AuthToken = "xoxb-9876543210123-4567778889990-f0A2GclR80dgPZLTUEq5asHm"

		conf.Database.URL = "postgres://username:password@host:5432/database-name"
		conf.Database.Encryption.Key = "01234abcde5678901234f62c898cdb592eb3166b56da733e8e798305b0ef6403"

		conf.Server.ClientID = "2518545982190.4321012345789"
		conf.Server.ClientSecret = "9f8e7dbc9a5ba2aa4522c7c8eb571f60"
		conf.Server.RedirectURL = "https://www.example.com/oidc/callback"
		conf.Server.SecretKey = "8c4faf836e29d282f2dc7ffdf4ef59c6081e2d8964ba0ac9cd4bc8800021300c"
		conf.Server.SigningSecret = "2773fb7eb76c90f19c0e1504ae1eee4b"

		return conf
	}

	tt := []test{
		{"default", newDefaultConfig(), true},
		{"valid", newValidConfig(), false},
		{"invalid database config", func() *Config {
			conf := newValidConfig()

			conf.Database.URL = "postgres://"
			return conf
		}(), true},
		{"invalid tracing config", func() *Config {
			conf := newValidConfig()

			conf.Tracing.Enabled = true
			conf.Tracing.Exporter = "x-ray"
			return conf
		}(), true},
		{"invalid server config", func() *Config {
			conf := newValidConfig()

			conf.Server.RedirectURL = "https://example.com/callback"
			return conf
		}(), true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.conf.Validate()

			if tc.isErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
