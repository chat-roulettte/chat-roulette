package o11y

import (
	"bytes"
	"context"
	"os"

	"github.com/hashicorp/go-hclog"

	"github.com/chat-roulettte/chat-roulette/internal/version"
)

// CreateLogger returns an annotated logger
func CreateLogger(level string, jsonFormat bool) (context.Context, hclog.Logger) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:           hclog.LevelFromString(level),
		Output:          os.Stdout,
		IncludeLocation: true,
		JSONFormat:      jsonFormat,
		Color:           hclog.AutoColor,
	})

	// Annotate the logger
	logger = logger.With(
		"service.name", ServiceName,
		"service.commit_sha", version.TruncatedCommitSha(),
		"service.build_date", version.BuildDate,
	)

	// Add the logger to the context
	ctx := hclog.WithContext(context.Background(), logger)

	return ctx, logger
}

// NewBufferedLogger returns a logger writing to a buffer.
//
// This should only be used in tests.
func NewBufferedLogger() (hclog.Logger, *bytes.Buffer) {
	buffer := bytes.NewBuffer(nil)

	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     buffer,
		JSONFormat: false,
	})

	return logger, buffer
}
