package database

import (
	"database/sql/driver"
	"encoding/json"
	"io"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bincyber/go-sqlcrypter"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

// AnyTime will return an sqlmock.Argument which can
// match any time.Time argument.
func AnyTime() sqlmock.Argument {
	return anyTime{}
}

type anyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a anyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

var _ sqlmock.Argument = (*anyTime)(nil)

// NewMockedGormDB returns a mocked gorm.DB for use in tests.
func NewMockedGormDB() (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, _ := sqlmock.New(sqlmock.MonitorPingsOption(true))

	gormDB, _ := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		DisableAutomaticPing: true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})

	return gormDB, mock
}

func MockQueueJob(mock sqlmock.Sqlmock, params interface{}, job string, priority int) {
	data, _ := json.Marshal(params)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "jobs" (.+) VALUES (.+) RETURNING`).
		WithArgs(
			sqlmock.AnyArg(),
			job,
			priority,
			models.JobStatusPending,
			false,
			string(data),
			AnyTime(),
			AnyTime(),
			AnyTime(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()
}

// NoOpCrypter is a no-op crypter useful for testing.
type NoOpCrypter struct{}

func (c NoOpCrypter) Encrypt(w io.Writer, r io.Reader) error {
	b, _ := io.ReadAll(r)
	w.Write(b) //nolint:errcheck
	return nil
}

func (c NoOpCrypter) Decrypt(w io.Writer, r io.Reader) error {
	b, _ := io.ReadAll(r)
	w.Write(b) //nolint:errcheck
	return nil
}

var _ sqlcrypter.Crypterer = (*NoOpCrypter)(nil)
