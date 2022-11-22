package database

import (
	"embed"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/pkg/errors"
)

//go:embed migrations/*.sql
var embeddedFS embed.FS

// Migrate leverages golang-migrate to perform a database migration
// using SQL migration files embedded within the binary.
func Migrate(databaseURL string) error {
	driver, err := iofs.New(embeddedFS, "migrations")
	if err != nil {
		return errors.Wrap(err, "failed to initialize golang-migrate iofs driver")
	}

	// The golang-migrate pkg uses lib/pg which defaults sslmode to "require",
	// therefore if sslmode is not set, we will set it to "disable" by default
	databaseURL = appendSSLModeDisable(databaseURL)

	m, err := migrate.NewWithSourceInstance("iofs", driver, databaseURL)
	if err != nil {
		return errors.Wrap(err, "failed to create new migrate instance")
	}

	if err := m.Up(); err != nil {
		// Don't error on "no change"
		if !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
	}

	return nil
}

// appendSSLModeDisable appends "sslmode=disable" to a
// database URL that does not have sslmode set.
func appendSSLModeDisable(databaseURL string) string {
	u, _ := url.Parse(databaseURL)

	values := u.Query()
	if _, ok := values["sslmode"]; !ok {
		values.Add("sslmode", "disable")
		u.RawQuery = values.Encode()
	}

	return u.String()
}
