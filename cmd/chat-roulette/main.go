package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/akamensky/argparse"
	"github.com/bincyber/go-sqlcrypter"
	"github.com/bincyber/go-sqlcrypter/providers/aesgcm"
	"go.opentelemetry.io/otel"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
	"github.com/chat-roulettte/chat-roulette/internal/server"
	"github.com/chat-roulettte/chat-roulette/internal/server/api/health"
	"github.com/chat-roulettte/chat-roulette/internal/server/api/oidc"
	apiv1 "github.com/chat-roulettte/chat-roulette/internal/server/api/v1"
	"github.com/chat-roulettte/chat-roulette/internal/server/ui"
	"github.com/chat-roulettte/chat-roulette/internal/worker"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	parser := argparse.NewParser("chat-roulette", "Chat Roulette for Slack")

	// Define flags
	configFile := parser.String("c", "config", &argparse.Options{
		Required: false,
		Help:     "the path to the config file",
	})

	migrateDatabase := parser.Flag("", "migrate", &argparse.Options{
		Required: false,
		Default:  false,
		Help:     "perform database migration on startup",
	})

	logLevel := parser.Selector("", "log.level",
		[]string{"info", "debug"},
		&argparse.Options{
			Required: false,
			Default:  "info",
			Help:     "the log level",
		})

	jsonLogging := parser.Flag("", "log.json", &argparse.Options{
		Required: false,
		Default:  false,
		Help:     "enable logging in JSON format",
	})

	if err := parser.Parse(os.Args); err != nil {
		log.Fatalf("failed to evaluate command-line flags: %s", err)
	}

	// Create logger
	ctx, logger := o11y.CreateLogger(*logLevel, *jsonLogging)

	// Read config
	if *configFile != "" {
		logger.Info(fmt.Sprintf("loading configuration from %s", *configFile))
	}

	conf, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		return err
	}
	logger.Debug("successfully loaded configuration")

	// Optionally perform database migration
	if *migrateDatabase {
		logger.Info("attempting database migration")
		if err := database.Migrate(conf.Database.URL); err != nil {
			logger.Error("failed to migrate database", "error", err)
			return err
		}
		logger.Info("successfully performed database migration")
	}

	// Create OpenTelemetry tracer
	if conf.Tracing.Enabled {
		tp, err := o11y.NewTracerProvider(&conf.Tracing)
		if err != nil {
			logger.Error("failed to configure tracing")
			return err
		}

		defer o11y.ShutdownTracer(ctx, logger, tp)
	}

	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "main")

	// Setup column-level encryption for sqlcrypter.EncryptedBytes
	key, err := conf.Database.Encryption.GetDEK()
	if err != nil {
		logger.Error("failed to read data encryption key")
		return err
	}

	previousKey, err := conf.Database.Encryption.GetPreviousDEK()
	if err != nil {
		logger.Error("failed to read previous data encryption key")
		return err
	}

	aesCrypter, err := aesgcm.New(key, previousKey)
	if err != nil {
		logger.Error("failed to initialize AES crypter", "error", err)
		return err
	}

	sqlcrypter.Init(aesCrypter)

	// Create the Server
	s, err := server.New(ctx, logger, conf)
	if err != nil {
		logger.Error("failed to create the Server", "error", err)
		return err
	}

	// Register API routes
	logger.Info("registering API routes")
	ui.RegisterRoutes(s)
	oidc.RegisterRoutes(s)
	health.RegisterRoutes(s)
	apiv1.RegisterRoutes(s)

	// Setup signal handlers for graceful shutdown
	stopCh := make(chan os.Signal, 2)
	errorCh := make(chan error)
	shutdownCh := make(chan bool)

	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// Create a wait group to manage graceful shutdown of all goroutines
	wg := new(sync.WaitGroup)

	// Create the worker
	w, err := worker.New(ctx, logger, conf, shutdownCh)
	if err != nil {
		logger.Error("failed to create worker", "error", err)
		return err
	}

	// Sync channels during startup
	if err := bot.QueueSyncChannelsJob(ctx, s.GetDB(), &bot.SyncChannelsParams{
		BotUserID: s.GetSlackBotUserID(),
	}); err != nil {
		logger.Error("failed to queue SYNC_CHANNELS job on startup", "error", err)
	}

	// End the span here before starting the HTTP server and worker(s)
	span.End()

	// Start the Server
	go s.Start(ctx, errorCh)

	// Start the Worker
	w.Start(ctx, wg)

	// Stop the Server when a signal is received
	wg.Add(1)
	s.Stop(ctx, wg, stopCh, errorCh)

	// Close the shutdown channel to stop all goroutines
	close(shutdownCh)

	// Wait for all goroutines to finish executing before quitting
	wg.Wait()

	logger.Info("exiting...")

	return nil
}
