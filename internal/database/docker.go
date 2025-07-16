package database

import (
	"database/sql"
	"net/url"
	"time"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"gorm.io/gorm"
)

// NewTestPostgresDB spawns a new Docker container running
// PostgreSQL v14.5 and executes database migration optionally.
//
// Use "defer resource.Close()" to ensure the container
// is purged when you are done with it.
func NewTestPostgresDB(migrate bool) (*dockertest.Resource, string, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, "", err
	}

	// Run Postgres v14.5 in a container
	user := "postgres"
	password := "letmein"
	db := "chat-roulette"

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "14.5",
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + db,
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.NeverRestart()
	})
	if err != nil {
		return nil, "", err
	}

	if err := resource.Expire(60); err != nil {
		return nil, "", err
	}

	dbURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Path:   db,
		Host:   resource.GetHostPort("5432/tcp"),
	}

	databaseURL := appendSSLModeDisable(dbURL.String())

	pool.MaxWait = 10 * time.Second
	if err = pool.Retry(func() error {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		return nil, "", err
	}

	// Migrate the database
	if migrate {
		if err := Migrate(databaseURL); err != nil {
			return nil, "", err
		}
	}

	return resource, databaseURL, nil
}

// CleanPostgresDB resets a running Postgres server to a clean state
// by dropping and recreating the public schema.
func CleanPostgresDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	if _, err := sqlDB.Exec("DROP SCHEMA public CASCADE;"); err != nil {
		return err
	}

	if _, err := sqlDB.Exec("CREATE SCHEMA public;"); err != nil {
		return err
	}

	return nil
}
