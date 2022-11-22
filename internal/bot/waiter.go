package bot

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

// waitOnMemberJobs waits for up to 30 seconds for all pending member
// modification jobs for a specific channel to complete before proceeding.
func waitOnMemberJobs(ctx context.Context, db *gorm.DB, channelID string) error {
	// Start a new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "job.wait")
	span.SetAttributes(
		attribute.String("slack_channel_id", channelID),
	)
	defer span.End()

	// The member jobs we care about
	jobTypes := []string{
		models.JobTypeAddMember.String(),
		models.JobTypeDeleteMember.String(),
		models.JobTypeUpdateMatch.String(),
	}

	// Wait up to 30 seconds
	t := time.NewTimer(30 * time.Second)

loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			break loop
		default:
			dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()

			var pendingJobs []string
			result := db.WithContext(dbCtx).
				Model(models.Job{}).
				Select("job_type").
				Where("data->>'channel_id' = ?", channelID).
				Where("is_completed = false").
				Where("job_type IN (?)", jobTypes).
				Find(&pendingJobs)

			if result.Error != nil {
				return errors.Wrap(result.Error, "failed to query pending member jobs")
			}

			if result.RowsAffected == 0 {
				break loop
			}
		}

		time.Sleep(time.Second)
	}

	return nil
}
