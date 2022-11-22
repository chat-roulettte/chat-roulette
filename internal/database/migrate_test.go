package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_appendDefaultSSLMode(t *testing.T) {
	t.Run("no change", func(t *testing.T) {
		url := "postgres://postgres:letmein@localhost:5432/chat-roulette?sslmode=require"
		result := appendSSLModeDisable(url)
		assert.NotContains(t, result, "?sslmode=disable")
	})

	t.Run("modified", func(t *testing.T) {
		url := "postgres://postgres:letmein@localhost:5432/chat-roulette"
		result := appendSSLModeDisable(url)
		assert.Contains(t, result, "?sslmode=disable")
	})
}

// This test leverages github.com/ory/dockertest
// to verify database migration works.
func Test_Migrate(t *testing.T) {
	r := require.New(t)

	resource, databaseURL, err := NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	// Migrate the database
	r.NoError(Migrate(databaseURL))

	// Migrate again to ensure that "no change" does not error
	r.NoError(Migrate(databaseURL))
}
