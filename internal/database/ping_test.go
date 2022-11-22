package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPing_success(t *testing.T) {
	resource, databaseURL, err := NewTestPostgresDB(false)
	require.NoError(t, err)
	defer resource.Close()

	db, err := NewGormDB(databaseURL)
	require.NoError(t, err)

	err = Ping(context.Background(), db)
	require.NoError(t, err)
}

func TestPing_failure(t *testing.T) {
	db, mock := NewMockedGormDB()
	mock.ExpectPing().WillReturnError(fmt.Errorf("connection error"))

	err := Ping(context.Background(), db)
	require.Error(t, err)

	require.Contains(t, err.Error(), "failed to ping the Postgres database")
}
