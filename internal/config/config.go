package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/chat-roulettte/chat-roulette/internal/isx"
)

// Config stores the configuration for the application
type Config struct {
	Bot          SlackBotConfig
	ChatRoulette ChatRouletteConfig
	Database     DatabaseConfig
	Server       ServerConfig
	Worker       WorkerConfig
	Tracing      TracingConfig
	Dev          bool
}

// DatabaseConfig stores the configuration for using the database
type DatabaseConfig struct {
	// URL is the PostgreSQL connection URL
	//
	// Required
	URL string

	// Connections is the database connections configuration
	//
	// Optional
	Connections DBConnectionsConfig

	// Encryption is the database encryption configuration
	//
	// Required
	Encryption DBEncryptionConfig
}

// DBEncryptionConfig stores the configuration for encrypting data in the database.
type DBEncryptionConfig struct {
	// Key is the current data encryption key (DEK) used to encrypt and decrypt data.
	// This must be a 32-byte hex-encoded key.
	//
	// Required
	Key string

	// PreviousKey is the key used to decrypt old data. This should
	// only be set if the data encryption key is being rotated.
	// If set, this must be a 32-byte hex-encoded key.
	//
	// Optional
	PreviousKey string `mapstructure:"previous_key"`
}

// GetDEK returns the hex-decoded DEK
func (d *DBEncryptionConfig) GetDEK() ([]byte, error) {
	key, err := hex.DecodeString(d.Key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hex decode data encryption key")
	}

	return key, nil
}

// GetDEK returns the hex-decoded previous DEK
func (d *DBEncryptionConfig) GetPreviousDEK() ([]byte, error) {
	if d.PreviousKey == "" {
		return nil, nil
	}

	key, err := hex.DecodeString(d.PreviousKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hex decode previous data encryption key")
	}

	return key, nil
}

// DBConnectionsConfig stores the configuration for the database connection pool
//
// See also: https://go.dev/doc/database/manage-connections
type DBConnectionsConfig struct {
	// MaxOpen is the maximum number of open connections.
	//
	// Optional
	MaxOpen int `mapstructure:"max_open"`

	// MaxIdle is the maximum number of idle connections.
	//
	// Optional
	MaxIdle int `mapstructure:"max_idle"`

	// MaxLifeTime is the maximum lifetime of connections.
	//
	// Optional
	MaxLifetime time.Duration `mapstructure:"max_lifetime"`

	// MaxIdleTime is the maximum amount of time a connection can be idle.
	//
	// Optional
	MaxIdletime time.Duration `mapstructure:"max_idletime"`
}

// ServerConfig stores the configuration for the bot server
type ServerConfig struct {
	// Address is the address that the server binds on.
	//
	// Optional, defaults to 0.0.0.0
	Address string

	// Port is the TCP port that the server binds on.
	//
	// Optional, defaults to 8080
	Port int

	// ClientID is the Slack OpenID Connect (OIDC) client ID.
	//
	// Required
	ClientID string `mapstructure:"client_id"`

	// Client Secret is Slack OpenID Connect (OIDC) client secret.
	//
	// Required
	ClientSecret string `mapstructure:"client_secret"`

	// RedirectURL is the Slack OpenID Connect (OIDC) redirect URL.
	//
	// Required
	RedirectURL string `mapstructure:"redirect_url"`

	// SecretKey is the 32-byte hex-encoded secret key used to authenticate cookies.
	//
	// Required
	SecretKey string `mapstructure:"secret_key"`

	// SigningSecret is the secret used to authenticate requests received from Slack.
	//
	// Required
	SigningSecret string `mapstructure:"signing_secret"`
}

// GetSecretKey returns the hex-decoded Secret Key
func (s *ServerConfig) GetSecretKey() ([]byte, error) {
	key, err := hex.DecodeString(s.SecretKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hex decode secret key")
	}

	return key, nil
}

// WorkerConfig stores the configuration for the task queue workers
type WorkerConfig struct {
	// Concurrency is the number of concurrent workers to run
	//
	// Optional, defaults to the number of CPU cores
	Concurrency int
}

// SlackBotConfig stores the configuration for the Slack bot
type SlackBotConfig struct {
	// AuthToken is the Slack OAuth2 bot token
	//
	// Required
	AuthToken string `mapstructure:"auth_token"`
}

// ChatRouletteConfig stores the configuration for chat roulette
type ChatRouletteConfig struct {
	// Interval is the interval or frequency that matches will be made.
	// Valid values are "weekly", "biweekly", "triweekly", "quadweekly", or "monthly".
	//
	// Optional
	//
	// Default: biweekly
	Interval string `mapstructure:"interval"`

	// Weekday is the day of the week that matches will be made.
	// eg, Monday
	//
	// Optional
	//
	// Default: Monday
	Weekday string `mapstructure:"weekday"`

	// Hour is the hour (in UTC) that matches will be made.
	//
	// Optional
	//
	// Default: 12
	Hour int `mapstructure:"hour"`

	// ConnectionMode is the mode (physical) of connections that will be made.
	// Valid values are "virtual", "physical", or "hybrid".
	//
	// Optional
	//
	// Default: virtual
	ConnectionMode string `mapstructure:"connection_mode"`
}

// TracingConfig stores the configuration for OpenTelemetry tracing.
// Only one exporter can be configured at a time.
type TracingConfig struct {
	// Enabled turns on OpenTelemetry tracing.
	//
	// Optional
	Enabled bool `mapstructure:"enabled"`

	// Exporter sets the tracing exporter to use.
	// The only options currently supported are: jaeger or honeycomb.
	//
	// Optional
	Exporter TracingExporter `mapstructure:"exporter"`

	// Jaeger stores the configuration for the Jaeger exporter
	//
	// Optional, required if Exporter=jaeger
	Jaeger JaegerTracing `mapstructure:"jaeger"`

	// Honeycomb stores the configuration for the Honeycomb exporter
	//
	// Optional, required if Exporter=honeycomb
	Honeycomb HoneycombTracing `mapstructure:"honeycomb"`
}

// JaegerTracing contains configuration for the Jaeger exporter
type JaegerTracing struct {
	// Endpoint is the URL of the Jaeger collector.
	// This is typically "http://localhost:4318/v1/traces"
	//
	// Required
	Endpoint string `mapstructure:"endpoint"`
}

// HoneycombTracing contains configuration for the Honeycomb exporter
type HoneycombTracing struct {
	// Team is the Honeycomb API key to use
	//
	// Required
	Team string `mapstructure:"team"`

	// Dataset is the destination dataset to send traces to
	//
	// Required
	Dataset string `mapstructure:"dataset"`
}

func newDefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Address: DefaultServerAddr,
			Port:    DefaultServerPort,
		},
		ChatRoulette: ChatRouletteConfig{
			Interval:       DefaultChatRouletteInterval,
			Weekday:        DefaultChatRouletteWeekday,
			Hour:           DefaultChatRouletteHour,
			ConnectionMode: DefaultChatRouletteConnectionMode,
		},
		Worker: WorkerConfig{
			Concurrency: DefaultWorkerConcurrency,
		},
		Database: DatabaseConfig{
			Connections: DBConnectionsConfig{
				MaxIdle:     DefaultDBMaxIdle,
				MaxOpen:     DefaultDBMaxOpen,
				MaxLifetime: DefaultDBMaxLifetime,
				MaxIdletime: DefaultDBMaxIdletime,
			},
		},
		Dev: false,
	}
}

// LoadConfig loads configuration from a file and environment variables
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	// Load default config
	config := newDefaultConfig()

	// Obtain config from environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	bindEnvs(v, config)

	// Obtain config from file, optionally
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	// PaaS typically set the PORT environment variable
	port := os.Getenv("PORT")
	if port != "" {
		v, _ := strconv.Atoi(port)
		config.Server.Port = v
	}

	// Validate the config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Normalize settings
	config.ChatRoulette.Interval = strings.ToLower(config.ChatRoulette.Interval)

	return &config, nil
}

// Validate verifies the configuration
func (c Config) Validate() error {
	// Validate bot config
	if err := validation.ValidateStruct(&c.Bot,
		validation.Field(&c.Bot.AuthToken, validation.By(isx.SlackBotAuthToken)),
	); err != nil {
		return errors.Wrap(err, "failed to validate bot config")
	}

	// Validate chat-roulette config
	if err := validation.ValidateStruct(&c.ChatRoulette,
		validation.Field(&c.ChatRoulette.Interval, validation.By(isx.Interval)),
		validation.Field(&c.ChatRoulette.Weekday, validation.By(isx.Weekday)),
		validation.Field(&c.ChatRoulette.Hour, validation.Required, validation.Min(0), validation.Max(23)),
		validation.Field(&c.ChatRoulette.ConnectionMode, validation.By(isx.ConnectionMode)),
	); err != nil {
		return errors.Wrap(err, "failed to validate chat-roulette config")
	}

	// Validate database config
	if err := validation.ValidateStruct(&c.Database,
		validation.Field(&c.Database.URL, validation.By(isx.PostgresConnectionURL)),
	); err != nil {
		return errors.Wrap(err, "failed to validate database URL")
	}

	if c.Database.Connections.MaxIdle > c.Database.Connections.MaxOpen {
		return fmt.Errorf("max_idle cannot be greater than max_open")
	}

	if err := validation.ValidateStruct(&c.Database.Connections,
		validation.Field(&c.Database.Connections.MaxOpen, validation.Min(5)),
		validation.Field(&c.Database.Connections.MaxIdle, validation.Min(2)),
		validation.Field(&c.Database.Connections.MaxIdletime, validation.Required),
		validation.Field(&c.Database.Connections.MaxLifetime, validation.Required),
	); err != nil {
		return errors.Wrap(err, "failed to validate database config")
	}

	if err := validation.ValidateStruct(&c.Database.Encryption,
		validation.Field(&c.Database.Encryption.Key, validation.Required, is.Hexadecimal, validation.Length(64, 64)),
		validation.Field(&c.Database.Encryption.PreviousKey, is.Hexadecimal, validation.Length(64, 64)),
	); err != nil {
		return errors.Wrap(err, "failed to validate database encryption config")
	}

	// Validate tracing config
	if c.Tracing.Enabled {
		switch c.Tracing.Exporter {
		case TracingExporterJaeger:
			if err := validation.ValidateStruct(&c.Tracing.Jaeger,
				validation.Field(&c.Tracing.Jaeger.Endpoint, validation.Required, is.URL),
			); err != nil {
				return errors.Wrap(err, "failed to validate Jaeger tracing config")
			}
		case TracingExporterHoneycomb:
			if err := validation.ValidateStruct(&c.Tracing.Honeycomb,
				validation.Field(&c.Tracing.Honeycomb.Team, validation.Required, is.Alphanumeric),
				validation.Field(&c.Tracing.Honeycomb.Dataset, validation.Required),
			); err != nil {
				return errors.Wrap(err, "failed to validate Honeycomb tracing config")
			}
		default:
			return errors.Wrap(fmt.Errorf("exporter can only be set to 'jaeger' or 'honeycomb'"), "failed to validate tracing config")
		}
	}

	// Validate server config
	if c.Server.Port < 0 || c.Server.Port > 65536 {
		return errors.Wrap(fmt.Errorf("invalid server port"), "failed to validate server config")
	}

	if err := validation.ValidateStruct(&c.Server,
		validation.Field(&c.Server.Address, validation.Required, is.Host),
		validation.Field(&c.Server.ClientID, validation.Required),
		validation.Field(&c.Server.ClientSecret, validation.Required),
		validation.Field(&c.Server.RedirectURL, validation.By(isx.RedirectURL)),
		validation.Field(&c.Server.SecretKey, validation.Required, is.Hexadecimal, validation.Length(64, 64)),
		validation.Field(&c.Server.SigningSecret, validation.Required),
	); err != nil {
		return errors.Wrap(err, "failed to validate server config")
	}

	// Validate worker config
	if c.Worker.Concurrency < 1 {
		return fmt.Errorf("worker concurrency cannot be less than 1")
	}

	if err := validation.ValidateStruct(&c.Worker); err != nil {
		return errors.Wrap(err, "failed to validate worker config")
	}

	return nil
}

// GetAddr returns the addr of the server
func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

// bindEnvs calls viper.BindEnv() for all fields of a struct
//
// This is a workaround for these viper issues:
// https://github.com/spf13/viper/issues/188
// https://github.com/spf13/viper/issues/761
func bindEnvs(v *viper.Viper, iface interface{}, parts ...string) {
	ifValue := reflect.ValueOf(iface)
	ifType := reflect.TypeOf(iface)

	for i := 0; i < ifType.NumField(); i++ {
		value := ifValue.Field(i)
		t := ifType.Field(i)

		tag, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			tag = strings.ToLower(t.Name)
		}

		switch value.Kind() {
		case reflect.Struct:
			bindEnvs(v, value.Interface(), append(parts, tag)...)
		default:
			v.BindEnv(strings.Join(append(parts, tag), ".")) //nolint:errcheck
		}
	}
}
