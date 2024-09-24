package isx

import (
	"testing"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
)

func Test_Interval(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validation.Validate("weekly", validation.By(Interval))

		assert.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		err := validation.Validate("blah", validation.By(Interval))

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "invalid chat roulette interval")
	})
}

func Test_Weekday(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validation.Validate("Friday", validation.By(Weekday))

		assert.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		err := validation.Validate("Sabbath", validation.By(Weekday))

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "invalid weekday")
	})
}

func Test_NextRoundDate(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		timestamp := time.Now().UTC().AddDate(0, 0, 3)

		err := validation.Validate(timestamp, validation.By(NextRoundDate))

		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		timestamp := time.Now().UTC().AddDate(0, 0, -2)

		err := validation.Validate(timestamp, validation.By(NextRoundDate))

		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid next round date")
	})
}

func Test_Country(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validation.Validate("Canada", validation.By(Country))

		assert.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		err := validation.Validate("Yugoslavia", validation.By(Country))

		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "invalid country")
	})
}

func Test_CalendlyLink(t *testing.T) {
	type test struct {
		name  string
		value string
		isErr bool
	}

	tt := []test{
		{"scheme", "https://calendly.com/bincyber", false},
		{"no scheme", "calendly.com/bincyber", false},
		{"empty", "", false},
		{"invalid domain", "twitter.com/bincyber", true},
		{"no user", "calendly.com/", true},
		{"ignore case", "HTTPS://Calendly.com/bincyber", false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.Validate(tc.value, validation.By(CalendlyLink))

			if tc.isErr {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "invalid Calendly link")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_ProfileType(t *testing.T) {
	type test struct {
		name  string
		isErr bool
	}

	tt := []test{
		{"Twitter", false},
		{"Instagram", false},
		{"Facebook", false},
		{"github", false},
		{"VK", true},
		{"Discord", true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.Validate(tc.name, validation.By(ProfileType))

			if tc.isErr {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "invalid profile type")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_ValidateProfileLink(t *testing.T) {
	type test struct {
		pType string
		pLink string
		isErr bool
	}

	tt := []test{
		{"Twitter", "twitter.com/joe", false},
		{"Instagram", "https://instagram.com/ahmed", false},
		{"LinkedIn", "facebook.com/bincyber", true},
		{"LinkedIn", "ca.linkedin.com/bincyber", false},
		{"github", "https://github.com/bincyber", false},
		{"twitter", "github.com/bincyber", true},
		{"tiktok", "tiktok.com/@example", false},
		{"pinterest", "p i n t e r e s t.com", true},
		{"snapchat", "m.snapchat.com", true},
		{"twitter", "twitter.com/", true},
	}

	for _, tc := range tt {
		t.Run(tc.pType, func(t *testing.T) {
			err := ValidProfileLink(tc.pType, tc.pLink)

			if tc.isErr {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "invalid profile link")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_RedirectURL(t *testing.T) {
	type test struct {
		name  string
		url   string
		isErr bool
	}

	tt := []test{
		{"valid", "https://example.com/oidc/callback", false},
		{"missing path", "https://example.com", true},
		{"invalid url", "h t t p s ://example.com", true},
		{"nil", "", true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.Validate(tc.url, validation.By(RedirectURL))

			if tc.isErr {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "invalid redirect URL")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_SlackBotAuthToken(t *testing.T) {
	type test struct {
		name  string
		value string
		isErr bool
	}

	tt := []test{
		{"valid", "xoxb-9876543210123-4567778889990-f0A2GclR80dgPZLTUEq5asHm", false},
		{"invalid", "xoxb-slack-bot-authtoken", true},
		{"malformed", "xoxb98765432101234567778889990f0A2GclR80dgPZLTUEq5asHm", true},
		{"nil", "", true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.Validate(tc.value, validation.By(SlackBotAuthToken))

			if tc.isErr {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "invalid bot auth token")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_PostgresConnectionURL(t *testing.T) {
	type test struct {
		name  string
		value string
		isErr bool
	}

	tt := []test{
		{
			"valid",
			"Postgres://username:password@host:5432/db-name",
			false,
		},
		{
			"invalid",
			"p o s t g r e s",
			true,
		},
		{
			"incorrect scheme",
			"mysql://username:password@host:5432/db-name",
			true,
		},
		{
			"missing username",
			"postgres://:password@host:5432/db-name",
			true,
		},
		{
			"missing password",
			"postgres://username@host:5432/db-name",
			true,
		},
		{
			"missing host",
			"postgres://username:password@/db-name",
			true,
		},
		{
			"missing port",
			"postgres://username:password@host/db-name",
			false,
		},
		{
			"missing database name",
			"postgres://username:password@host:5432",
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.Validate(tc.value, validation.By(PostgresConnectionURL))

			if tc.isErr {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "invalid PostgreSQL connection URL")
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
