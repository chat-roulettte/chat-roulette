package models

import (
	"time"

	"github.com/bincyber/go-sqlcrypter"
	"github.com/segmentio/ksuid"
	"gorm.io/datatypes"
)

// Channel represents a row in the channels table
type Channel struct {
	// ChannelID is the ID of the Slack channel
	ChannelID string `gorm:"primaryKey"`

	// Inviter is the ID of the user who has invited the bot to the Slack channel
	Inviter string

	// ConnectionMode ...
	ConnectionMode ConnectionMode

	// Interval is the interval for chat roulette rounds for the channel (ie. weekly, biweekly, triweekly, monthly)
	Interval IntervalEnum

	// Weekday is the weekday in which new chat roulette rounds are started for the channel (ie. Sunday, Monday, etc.)
	Weekday time.Weekday

	// Hour is the hour in which new chat roulette rounds are started for the channel (ie. 10, 12, 18)
	Hour int

	// NextRound is the timestamp of the next chat roulette round
	NextRound time.Time

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time

	// UpdatedAt is the timestamp of when the record was last updated
	UpdatedAt time.Time
}

// Member represents a row in the members table
type Member struct {
	// ID is the primary key for the table
	ID int32 `gorm:"primaryKey"`

	// UserID is the ID of the Slack user
	UserID string

	// ChannelID is the ID of the Slack channel that the user is a member of
	ChannelID string `gorm:"foreignKey:ChannelID;references:Channel"`

	// Gender is the gender of the user
	Gender Gender

	// Country is the country in which the Slack user resides
	Country sqlcrypter.EncryptedBytes

	// City is the city in which the Slack user resides
	City sqlcrypter.EncryptedBytes

	// Timezone is the timezone that the Slack user is in
	Timezone sqlcrypter.EncryptedBytes

	// ProfileType is the Slack user's social profile type
	ProfileType sqlcrypter.EncryptedBytes

	// ProfileLink is the link to the Slack user's social profile
	ProfileLink sqlcrypter.EncryptedBytes

	// CalendlyLink is a link for the user's Calendly
	CalendlyLink sqlcrypter.EncryptedBytes

	// IsActive is a boolean flag for if the user is participating in chat roulette
	//
	// A pointer is used here to ensure non-zero value (ie. false) is saved.
	IsActive *bool

	// HasGenderPreference is a boolean flag for if the user wishes to only be matched
	// with other participants of the same gender.
	//
	// A pointer is used here to ensure non-zero value (ie. false) is saved.
	HasGenderPreference *bool

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time

	// UpdatedAt is the timestamp of when the record was last updated
	UpdatedAt time.Time
}

// Round represents a row in the rounds table
type Round struct {
	// ID is the primary key for the table
	ID int32 `gorm:"primaryKey"`

	// ChannelID is the ID of the Slack channel for this round
	ChannelID string `gorm:"foreignKey:ChannelID;references:Channel"`

	// HasEnded is a boolean flag for if the chat roulette round has concluded
	HasEnded bool

	// InactiveParticipants tracks how many participants were marked as inactive during this round
	InactiveParticipants int16 `gorm:"column:inactive_users"`

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time

	// UpdatedAt is the timestamp of when the record was last updated
	UpdatedAt time.Time
}

// Match represents a row in the matches table
type Match struct {
	// ID is the primary key for the table
	ID int32 `gorm:"primaryKey"`

	// RoundID is the ID of the chat roulette round
	RoundID int32 `gorm:"foreignKey:RoundID;references:Round"`

	// MpimID is the ID of the Slack group DM
	MpimID string `gorm:"column:mpim_id"`

	// HasMet is a boolean flag for if the match has met for chat roulette
	HasMet bool

	// WasNotified is a boolean flag for if the match has been notified
	WasNotified bool

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time

	// UpdatedAt is the timestamp of when the record was last updated
	UpdatedAt time.Time
}

// Pairing represents a row in the pairings table
type Pairing struct {
	// ID is an auto-incrementing identifier for the table
	ID int32 `gorm:"autoIncrement"`

	// MatchID is the ID of the chat roulette match
	MatchID int32 `gorm:"primaryKey;foreignKey:MatchID;references:Match"`

	// MemberID is the ID of the Slack user in this chat roulette match
	MemberID int32 `gorm:"primaryKey;foreignKey:MemberID;references:Members"`

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time
}

// Icebreaker represents a row in the icebreakers table
type Icebreaker struct {
	// ID is an auto-incrementing identifier for the table
	ID int32 `gorm:"autoIncrement"`

	// Question is the icebreaker question
	Question string

	// UsageCount is the count of how many times this icebreaker has been selected
	UsageCount int32 `gorm:"column:usage_count"`

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time

	// UpdatedAt is the timestamp of when the record was last updated
	UpdatedAt time.Time
}

// Job represents a row in the jobs table
type Job struct {
	// ID is the primary key of the record
	ID int32 `gorm:"primaryKey"`

	// JobID is the unique ID for the job
	JobID ksuid.KSUID `gorm:"column:job_id"`

	// JobType is the type of job
	JobType jobTypeEnum

	// Priority is the execution priority (1 - 10) for the job in the queue, with 10 being highest
	Priority int

	// Status is the completion status (success, failed, etc.) for a job
	Status jobStatusEnum

	// IsCompleted is a boolean flag for checking if the job has been completed regardless of completion status
	IsCompleted bool

	// Data is the JSON data for the job
	Data datatypes.JSON

	// ExecAt is the timestamp of when the job should be executed
	ExecAt time.Time

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time

	// UpdatedAt is the timestamp of when the record was last updated
	UpdatedAt time.Time
}

// NewJob creates a new *Job that will be scheduled with standard execution priority.
func NewJob(jobType jobTypeEnum, data datatypes.JSON) *Job {
	return &Job{
		JobID:       ksuid.New(),
		JobType:     jobType,
		Priority:    JobPriorityStandard,
		Status:      JobStatusPending,
		IsCompleted: false,
		Data:        data,
		ExecAt:      time.Now().UTC(),
	}
}

// BlockedMember represents a row in the blocked_members table
type BlockedMember struct {
	// ID is the primary key for the table
	ID int32 `gorm:"primaryKey"`

	// ChannelID is the ID of the Slack channel that the user is a member of
	ChannelID string `gorm:"foreignKey:ChannelID;references:Channel"`

	// UserID is the ID of the Slack user who is doing the blocking
	UserID string `gorm:"foreignKey:UserID;references:User"`

	// MemberID is the ID of the Slack user who is blocked from being matched with UserID
	MemberID string

	// CreatedAt is the timestamp of when the record was first created
	CreatedAt time.Time
}
