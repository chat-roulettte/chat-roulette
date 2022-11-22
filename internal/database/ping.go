package database

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// Ping pings the database to test the connection
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, _ := db.DB()

	if err := sqlDB.PingContext(ctx); err != nil {
		return errors.Wrap(err, "failed to ping the Postgres database")
	}

	return nil
}
