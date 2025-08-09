package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/ory/dockertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_execJob(t *testing.T) {
	j := &models.Job{
		JobType: models.JobTypeUnknown,
	}

	w := Worker{}

	err := w.execJob(context.Background(), j, nil)
	assert.Error(t, err)
}

type ProcessJobTestSuite struct {
	suite.Suite
	worker   Worker
	ctx      context.Context
	resource *dockertest.Resource
	db       *gorm.DB
	logger   hclog.Logger
	buffer   *bytes.Buffer
}

func (s *ProcessJobTestSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)

	resource, databaseURL, err := database.NewTestPostgresDB(true)
	if err != nil {
		log.Fatal(err)
	}
	s.resource = resource

	// Setup the Worker
	db, err := database.NewGormDB(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	s.db = db

	s.worker = Worker{
		id:     "test",
		db:     db,
		logger: s.logger,
	}
}

func (s *ProcessJobTestSuite) TearDownTest() {
	require.NoError(s.T(), database.CleanPostgresDB(s.db))
}

func (s *ProcessJobTestSuite) TearDownSuite() {
	s.resource.Close()
}

func (s *ProcessJobTestSuite) Test_Succeeded() {
	r := require.New(s.T())

	p := bot.AddChannelParams{
		ChannelID:      "C0123456789",
		Inviter:        "U9876543210",
		ConnectionMode: "virtual",
		Interval:       "weekly",
		Weekday:        "Friday",
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	}

	data, _ := json.Marshal(p)
	job := models.NewJob(models.JobTypeAddChannel, data)
	s.db.Save(&job)

	err := s.worker.processJob(s.ctx, trace.Link{})
	r.NoError(err)

	// Verify job completed successfully
	r.Contains(s.buffer.String(), "added Slack channel to the database")
	result := s.db.First(&job)
	r.NoError(result.Error)
	r.Equal(result.RowsAffected, int64(1))
	r.Equal(job.Status, models.JobStatusSucceeded)
	r.True(job.IsCompleted)
}

func (s *ProcessJobTestSuite) Test_Failed_MissingSlackChannel() {
	r := require.New(s.T())

	job := models.NewJob(models.JobTypeAddMember, []byte(`{"foo":"bar"}`))
	s.db.Save(&job)

	err := s.worker.processJob(context.Background(), trace.Link{})
	r.NoError(err)

	// Verify job was marked as failed
	r.Contains(s.buffer.String(), "failed to extract Slack channel ID from job data")
	result := s.db.First(&job)
	r.NoError(result.Error)
	r.Equal(result.RowsAffected, int64(1))
	r.Equal(job.Status, models.JobStatusFailed)
	r.True(job.IsCompleted)
}

func (s *ProcessJobTestSuite) Test_Failed_ValidationError() {
	r := require.New(s.T())

	job := models.NewJob(models.JobTypeBlockMember, []byte(`{"user_id":"1 2 3","member_id":"U8765432109"}`))
	s.db.Save(&job)

	err := s.worker.processJob(context.Background(), trace.Link{})
	r.Error(err)

	// Verify job was marked as failed
	r.Contains(s.buffer.String(), "failed to execute job")
	result := s.db.First(&job)
	r.NoError(result.Error)
	r.Equal(result.RowsAffected, int64(1))
	r.Equal(job.Status, models.JobStatusFailed)
	r.True(job.IsCompleted)
}

func (s *ProcessJobTestSuite) Test_Canceled() {
	r := require.New(s.T())

	p := bot.AddMemberParams{
		ChannelID: "C0123456789",
		UserID:    "U9876543210",
	}

	data, _ := json.Marshal(p)
	job := models.NewJob(models.JobTypeAddMember, data)
	s.db.Save(&job)

	err := s.worker.processJob(context.Background(), trace.Link{})
	r.Error(err)

	// Verify job was canceled
	r.Contains(s.buffer.String(), "Slack channel does not exist")
	result := s.db.First(&job)
	r.NoError(result.Error)
	r.Equal(result.RowsAffected, int64(1))
	r.Equal(job.Status, models.JobStatusCanceled)
	r.True(job.IsCompleted)
}

func Test_ProcessJob_suite(t *testing.T) {
	suite.Run(t, new(ProcessJobTestSuite))
}
