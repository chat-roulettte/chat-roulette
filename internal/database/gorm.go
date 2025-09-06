package database

import (
	"log"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/chat-roulettte/chat-roulette/internal/config"
)

// CreateGormDB creates a configured gorm.DB
func CreateGormDB(logger hclog.Logger, c *config.Config) (*gorm.DB, error) {
	// Disable logging unless running in Dev mode, traces are sufficient
	logLevel := gormlogger.Silent

	if c.Dev {
		logLevel = gormlogger.Info
	}

	stdLogger := logger.StandardLogger(&hclog.StandardLoggerOptions{
		InferLevels: true,
	})

	// Connect gorm to Postgres database
	db, err := gorm.Open(postgres.Open(c.Database.URL), &gorm.Config{
		DisableAutomaticPing: true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		DefaultContextTimeout: 1 * time.Second,
		Logger: gormlogger.New(stdLogger, gormlogger.Config{
			SlowThreshold:             100 * time.Millisecond,
			LogLevel:                  logLevel,
			Colorful:                  true,
			IgnoreRecordNotFoundError: false,
		}),
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize connection to Postgres database")
	}

	// Configure gorm to export OpenTelemetry spans
	var opts []otelgorm.Option
	if !c.Dev {
		opts = append(opts, otelgorm.WithoutQueryVariables())
	}

	if err := db.Use(otelgorm.NewPlugin(opts...)); err != nil {
		return nil, errors.Wrap(err, "failed to configure gorm with OpenTelemetry plugin")
	}

	// Enable SQL debug logging if running in Dev Mode
	if c.Dev {
		db = db.Debug()
	}

	// Tune connection pooling settings
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(c.Database.Connections.MaxOpen)
	sqlDB.SetMaxIdleConns(c.Database.Connections.MaxIdle)
	sqlDB.SetConnMaxLifetime(c.Database.Connections.MaxLifetime)
	sqlDB.SetConnMaxIdleTime(c.Database.Connections.MaxIdletime)

	return db, nil
}

// NewGormDB creates a gorm.DB given a database URL.
//
// This is a utility function only to be used for tests.
func NewGormDB(databaseURL string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		DisableAutomaticPing: true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		Logger: gormlogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             100 * time.Millisecond,
				LogLevel:                  gormlogger.Warn,
				Colorful:                  false,
				IgnoreRecordNotFoundError: true,
			}),
	})
}
