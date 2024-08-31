package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/segmentio/ksuid"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/slackclient"
)

// Worker works on jobs in the queue
type Worker struct {
	// id of the worker
	id string

	// logger ...
	logger hclog.Logger

	// db ...
	db *gorm.DB

	// slackClient ...
	slackClient *slack.Client

	// interval is the frequency to process jobs
	interval time.Duration

	// concurrency is the number of jobs to process at a time
	concurrency int

	// shutdownCh is the channel that is closed to stop the worker
	shutdownCh <-chan bool
}

// New creates a new Worker
func New(ctx context.Context, logger hclog.Logger, c *config.Config, ch <-chan bool) (*Worker, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "worker.create")
	defer span.End()

	// Generate a unique ID for the worker
	workerID := ksuid.New().String()

	logger = logger.With("worker_id", workerID)

	span.SetAttributes(
		attribute.String("worker_id", workerID),
	)

	// Configure gorm.DB
	db, err := database.CreateGormDB(logger, c)
	if err != nil {
		logger.Error("failed to create gorm.DB", "error", err)
		return nil, err
	}

	// Create Slack client
	slackClient, _ := slackclient.New(logger, c.Bot.AuthToken)

	w := &Worker{
		id:          workerID,
		logger:      logger,
		db:          db,
		slackClient: slackClient,
		interval:    1 * time.Second,
		concurrency: c.Worker.Concurrency,
		shutdownCh:  ch,
	}

	return w, nil
}

// Start starts the worker running in the background with the desired concurrency
func (w *Worker) Start(ctx context.Context, wg *sync.WaitGroup) {
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go w.run(ctx, wg)
	}
}

// run runs the control loop for the worker and gracefully stops when the shutdown channel is closed
func (w *Worker) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Extract the link from the parent trace
	link := trace.LinkFromContext(ctx)

	// This context will be canceled when the shutdown channel is closed
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Jobs will be processed in the queue on this interval
	ticker := time.NewTicker(w.interval)

	for {
		select {
		case <-w.shutdownCh:
			w.logger.Info("received shutdown, gracefully stopping Worker")
			cancel()
			ticker.Stop()
			return

		case <-ticker.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				w.processJob(ctx, link) //nolint:errcheck
			}()
		}
	}
}

// processJob gets the next available job from the queue and handles it
func (w *Worker) processJob(ctx context.Context, link trace.Link) error {
	// Start a new root span linked to the parent trace
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "worker.run", trace.WithLinks(link), trace.WithNewRoot())
	span.SetAttributes(
		attribute.String("worker_id", w.id),
	)
	defer span.End()

	// Within a transaction: query for available jobs, get an exclusive lock on the job, and work on it
	tx := w.db.Begin()
	if tx.Error != nil {
		w.logger.Debug("failed to begin db transaction", "error", tx.Error)
		return tx.Error
	}

	childCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	job := new(models.Job)

	// GetNextJob() is a custom Postgres function. Refer to SQL migration files
	result := tx.WithContext(childCtx).Raw("SELECT * FROM GetNextJob()").First(&job)

	if result.Error != nil {
		tx.Rollback()

		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			w.logger.Error("failed to query for available jobs", "error", result.Error)
		}

		return result.Error
	}

	span.SetAttributes(
		attribute.String("job", job.JobType.String()),
		attribute.String("job_id", job.JobID.String()),
	)

	// Since almost all jobs require the Slack channel to exist in the database,
	// perform that check here instead of within each job. If the Slack channel
	// does not exist, mark the job as canceled and skip it.
	if models.JobRequiresSlackChannel(job.JobType) {
		// Since the params of these jobs must include the Slack channel ID,
		// we can unmarshal the JSON to get the Slack channel ID
		var p *bot.SyncMembersParams

		err := json.Unmarshal(job.Data, &p)
		if err != nil || p.ChannelID == "" {
			w.logger.Warn("failed to unmarshal JSON and extract Slack channel ID", "error", err)

			job.Status = models.JobStatusFailed
			job.IsCompleted = true
			tx.WithContext(ctx).Save(&job)
			tx.Commit()

			span.SetAttributes(
				attribute.String("job_status", job.Status.String()),
			)

			return fmt.Errorf("failed to unmarshal JSON and extract Slack channel ID")
		}

		// Check if the Slack channel exists
		dbCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		defer cancel()

		result := tx.WithContext(dbCtx).
			Model(&models.Channel{}).
			Select("channel_id").
			Where("channel_id = ?", p.ChannelID).
			First(&models.Channel{})

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			w.logger.Warn("Slack channel does not exist in the database")

			job.Status = models.JobStatusCanceled
			job.IsCompleted = true
			tx.WithContext(ctx).Save(&job)
			tx.Commit()

			span.SetAttributes(
				attribute.String("job_status", job.Status.String()),
			)

			return result.Error
		}
	}

	// Execute the job
	w.logger.Info("executing job", "job_id", job.JobID.String(), "job", job.JobType.String())

	if err := w.execJob(ctx, job, tx); err != nil {
		w.logger.Error("failed to execute job",
			"error", err,
			"job_id", job.JobID.String(),
			"job", job.JobType.String(),
		)

		span.SetAttributes(
			attribute.String("job_status", models.JobStatusErrored.String()),
		)

		tx.Rollback()
		return err
	}

	job.Status = models.JobStatusSucceeded
	job.IsCompleted = true
	tx.WithContext(ctx).Save(&job)
	tx.Commit()

	span.SetAttributes(
		attribute.String("job_status", job.Status.String()),
	)

	return nil
}

// execJob executes the job
func (w *Worker) execJob(ctx context.Context, job *models.Job, tx *gorm.DB) error {
	// Start a new span
	ctx, span := otel.Tracer("").Start(ctx, "worker.exec")
	span.SetAttributes(
		attribute.String("worker_id", w.id),
		attribute.String("job", job.JobType.String()),
		attribute.String("job_id", job.JobID.String()),
	)
	defer span.End()

	var err error

	switch job.JobType {
	case models.JobTypeAddChannel:
		err = bot.ExecJob(ctx, tx, nil, job, bot.AddChannel)

	case models.JobTypeSyncChannels:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.SyncChannels)

	case models.JobTypeUpdateChannel:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.UpdateChannel)

	case models.JobTypeDeleteChannel:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.DeleteChannel)

	case models.JobTypeSyncMembers:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.SyncMembers)

	case models.JobTypeAddMember:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.AddMember)

	case models.JobTypeUpdateMember:
		err = bot.ExecJob(ctx, tx, nil, job, bot.UpdateMember)

	case models.JobTypeDeleteMember:
		err = bot.ExecJob(ctx, tx, nil, job, bot.DeleteMember)

	case models.JobTypeGreetMember:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.GreetMember)

	case models.JobTypeCreateRound:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.CreateRound)

	case models.JobTypeEndRound:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.EndRound)

	case models.JobTypeCreateMatches:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.CreateMatches)

	case models.JobTypeUpdateMatch:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.UpdateMatch)

	case models.JobTypeCreatePair:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.CreatePair)

	case models.JobTypeNotifyPair:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.NotifyPair)

	case models.JobTypeNotifyMember:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.NotifyMember)

	case models.JobTypeCheckPair:
		err = bot.ExecJob(ctx, nil, w.slackClient, job, bot.CheckPair)

	case models.JobTypeReportStats:
		err = bot.ExecJob(ctx, tx, w.slackClient, job, bot.ReportStats)

	default:
		err = fmt.Errorf("invalid job type")
	}

	return err
}
