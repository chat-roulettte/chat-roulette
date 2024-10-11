package bot

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// JobFunc is the function signature for all job functions.
type JobFunc[T any] func(ctx context.Context, db *gorm.DB, client *slack.Client, p *T) error

// QueueJob is a generic function for adding a background job to the database job queue.
func QueueJob[T any](ctx context.Context, db *gorm.DB, gJob models.GenericJob[T]) error {
	// Start a new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "job.queue")
	defer span.End()

	logger := hclog.FromContext(ctx)

	// Annotate the span and logger with the Slack channel ID
	channelID := extractChannelIDFromParams(gJob.Params)
	if channelID != "" {
		span.SetAttributes(
			attribute.String(attributes.SlackChannelID, channelID),
		)

		logger = logger.With(attributes.SlackChannelID, channelID)
	}

	// Add the new job to the database
	data, err := json.Marshal(gJob.Params)
	if err != nil {
		logger.Error("failed to marshal JSON", "error", err)
		return err
	}

	job := models.NewJob(gJob.JobType, data)

	if gJob.Priority != 0 {
		job.Priority = gJob.Priority
	}

	if !gJob.ExecAt.IsZero() {
		job.ExecAt = gJob.ExecAt
	}

	span.SetAttributes(
		attribute.String(attributes.JobType, job.JobType.String()),
		attribute.String(attributes.JobID, job.JobID.String()),
		attribute.Int(attributes.JobPriority, job.Priority),
	)

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := db.WithContext(dbCtx).Create(job).Error; err != nil {
		logger.Error("failed to add new job to the database", "error", err)
		return err
	}

	logger.Info("added new job to the database", "job_id", job.JobID)

	return nil
}

// ExecJob is a generic function for executing job functions.
func ExecJob[T any](ctx context.Context, db *gorm.DB, client *slack.Client, job *models.Job, f JobFunc[T]) error {
	// Start a new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "job.exec")
	defer span.End()

	span.SetAttributes(
		attribute.String(attributes.JobType, job.JobType.String()),
		attribute.String(attributes.JobID, job.JobID.String()),
		attribute.Int(attributes.JobPriority, job.Priority),
	)

	// Inject annotated logger into the context for the job function below
	ctx = hclog.WithContext(ctx, hclog.FromContext(ctx).With(
		attributes.JobType, job.JobType.String(),
		attributes.JobID, job.JobID.String(),
	))

	// Unmarshal JSONB to params for the job
	params := new(T)
	if err := json.Unmarshal(job.Data, &params); err != nil {
		return err
	}

	// Annotate the span with the Slack channel ID
	channelID := extractChannelIDFromParams(params)
	if channelID != "" {
		span.SetAttributes(
			attribute.String(attributes.SlackChannelID, channelID),
		)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return f(ctx, db, client, params)
}

// extractChannelIDFromParams attempts to extract the
// ChannelID from a job's Params struct using reflection.
func extractChannelIDFromParams[T any](p T) string {
	var channelID string

	pType := reflect.TypeOf(p)
	pValue := reflect.ValueOf(p)

	if pType.Kind() == reflect.Struct {
		field, ok := pType.FieldByName("ChannelID")
		if !ok {
			return channelID
		}

		channelID = pValue.FieldByIndex(field.Index).String()
	}

	return channelID
}
