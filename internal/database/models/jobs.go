package models

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// JobPriority are the priorities to use for jobs
	JobPriorityHighest  = 10
	JobPriorityHigh     = 8
	JobPriorityStandard = 5
	JobPriorityLow      = 3
	JobPriorityLowest   = 1
)

var (
	// ErrInvalidJobType is returned when an invalid job type is used
	ErrInvalidJobType = errors.New("invalid job type")

	// ErrInvalidJobStatus is returned when an invalid job status is used
	ErrInvalidJobStatus = errors.New("invalid job status")
)

// jobTypeEnum is an enum for the various jobs in the queue
type jobTypeEnum int64

const (
	// JobTypeUnknown is reserved and unused
	JobTypeUnknown jobTypeEnum = iota

	// JobTypeAddChannel is the job for adding a new Slack channel
	JobTypeAddChannel

	// JobTypeGreetAdmin is the job for greeting the admin of a new Slack channel
	JobTypeGreetAdmin

	// JobTypeUpdateChannel is the job for updating settings for a Slack channel
	JobTypeUpdateChannel

	// JobTypeDeleteChannel is the job for deleting a Slack channel
	JobTypeDeleteChannel

	// JobTypeSyncChannels is the job for syncing Slack channels with the database
	JobTypeSyncChannels

	// JobTypeAddMember is the job for adding a new Slack member for a channel
	JobTypeAddMember

	// JobTypeUpdateMember is the job for updating a Slack member for a channel
	JobTypeUpdateMember

	// JobTypeGreetMember is the job for greeting a new Slack member for a channel
	JobTypeGreetMember

	// JobTypeDeleteMember is the job for deleting a Slack member for a channel
	JobTypeDeleteMember

	// JobTypeSyncMembers is the job for syncing Slack members with the database
	JobTypeSyncMembers

	// JobTypeCreateRound is the job for starting a new chat roulette round for a channel
	JobTypeCreateRound

	// JobTypeEndRound is the job for ending a running chat roulette round for a channel
	JobTypeEndRound

	// JobTypeCreateMatches is the job for creating matches for a round of chat roulette
	JobTypeCreateMatches

	// JobTypeReportMatch is the job for reporting matches for a round of chat roulette
	JobTypeReportMatches

	// JobTypeCreateMatch is the job for creating a single match for a round of chat roulette
	JobTypeCreateMatch

	// JobTypeUpdateMatch is the job for updating the status of a match at the end of a round of chat roulette
	JobTypeUpdateMatch

	// JobTypeCreatePair is the job for creating a pair for a round of chat roulette
	JobTypeCreatePair

	// JobTypeNotifyPair is the job for notifying a pair for a round of chat roulette
	JobTypeNotifyPair

	// JobTypeKickoffPair is the job for kickstarting conversation for a pair for a round of chat roulette
	JobTypeKickoffPair

	// JobTypeNotifyMember is the job for notifying a Slack member that they have not been matched in a round of chat roulette
	JobTypeNotifyMember

	// JobTypeCheckPair is the job for checking if a pair has met for a round of chat roulette
	JobTypeCheckPair

	// JobTypeReportStats is the job for generating report for a chat roulette round
	JobTypeReportStats

	// JobTypeMarkInactive is the job for marking users as inactive
	JobTypeMarkInactive
)

var jobTypes = map[string]jobTypeEnum{
	"ADD_CHANNEL":    JobTypeAddChannel,
	"UPDATE_CHANNEL": JobTypeUpdateChannel,
	"DELETE_CHANNEL": JobTypeDeleteChannel,
	"SYNC_CHANNELS":  JobTypeSyncChannels,
	"GREET_ADMIN":    JobTypeGreetAdmin,
	"ADD_MEMBER":     JobTypeAddMember,
	"GREET_MEMBER":   JobTypeGreetMember,
	"UPDATE_MEMBER":  JobTypeUpdateMember,
	"DELETE_MEMBER":  JobTypeDeleteMember,
	"SYNC_MEMBERS":   JobTypeSyncMembers,
	"CREATE_ROUND":   JobTypeCreateRound,
	"END_ROUND":      JobTypeEndRound,
	"CREATE_MATCHES": JobTypeCreateMatches,
	"REPORT_MATCHES": JobTypeReportMatches,
	"CREATE_MATCH":   JobTypeCreateMatch,
	"UPDATE_MATCH":   JobTypeUpdateMatch,
	"CREATE_PAIR":    JobTypeCreatePair,
	"NOTIFY_PAIR":    JobTypeNotifyPair,
	"KICKOFF_PAIR":   JobTypeKickoffPair,
	"NOTIFY_MEMBER":  JobTypeNotifyMember,
	"CHECK_PAIR":     JobTypeCheckPair,
	"REPORT_STATS":   JobTypeReportStats,
	"MARK_INACTIVE":  JobTypeMarkInactive,
}

func (j jobTypeEnum) String() string {
	switch j {
	case JobTypeAddChannel:
		return "ADD_CHANNEL"
	case JobTypeUpdateChannel:
		return "UPDATE_CHANNEL"
	case JobTypeDeleteChannel:
		return "DELETE_CHANNEL"
	case JobTypeSyncChannels:
		return "SYNC_CHANNELS"
	case JobTypeGreetAdmin:
		return "GREET_ADMIN"
	case JobTypeAddMember:
		return "ADD_MEMBER"
	case JobTypeGreetMember:
		return "GREET_MEMBER"
	case JobTypeUpdateMember:
		return "UPDATE_MEMBER"
	case JobTypeDeleteMember:
		return "DELETE_MEMBER"
	case JobTypeSyncMembers:
		return "SYNC_MEMBERS"
	case JobTypeCreateRound:
		return "CREATE_ROUND"
	case JobTypeEndRound:
		return "END_ROUND"
	case JobTypeCreateMatches:
		return "CREATE_MATCHES"
	case JobTypeReportMatches:
		return "REPORT_MATCHES"
	case JobTypeCreateMatch:
		return "CREATE_MATCH"
	case JobTypeUpdateMatch:
		return "UPDATE_MATCH"
	case JobTypeCreatePair:
		return "CREATE_PAIR"
	case JobTypeKickoffPair:
		return "KICKOFF_PAIR"
	case JobTypeNotifyPair:
		return "NOTIFY_PAIR"
	case JobTypeNotifyMember:
		return "NOTIFY_MEMBER"
	case JobTypeCheckPair:
		return "CHECK_PAIR"
	case JobTypeReportStats:
		return "REPORT_STATS"
	case JobTypeMarkInactive:
		return "MARK_INACTIVE"
	default:
		return ""
	}
}

// Scan implements the Scanner interface
func (j *jobTypeEnum) Scan(value interface{}) error {
	s, _ := value.(string)
	s = strings.ToUpper(s)

	if v, ok := jobTypes[s]; ok {
		*j = v
		return nil
	}

	return ErrInvalidJobType
}

// Value implements the Valuer interface
func (j jobTypeEnum) Value() (driver.Value, error) {
	v := j.String()
	if v != "" {
		return driver.Value(v), nil
	}

	return nil, ErrInvalidJobType
}

var (
	_ driver.Valuer = (*jobTypeEnum)(nil)
	_ sql.Scanner   = (*jobTypeEnum)(nil)
)

// GenericJob is the generic representation of a background job
// with job parameters, priority, and execution time.
type GenericJob[T any] struct {
	// JobType is the type of background job.
	JobType jobTypeEnum
	// Params are the parameters for the job.
	// This must match the job type.
	Params T
	// Priority is the priority in the queue.
	// This is optional and will default to JobPriorityStandard.
	Priority int
	// ExecAt is the execution time.
	// This is optional and will default to time.Now().
	ExecAt time.Time
}

// jobStatusEnum is an enum for the completion status for jobs
type jobStatusEnum int64

const (
	// JobStatusUnknown is reserved and unused
	JobStatusUnknown jobStatusEnum = iota

	// JobStatusPending is the default status for new jobs and jobs waiting to be retried
	JobStatusPending

	// JobStatusErrored is the status for a job that has hit an error, but can be retried
	JobStatusErrored

	// JobStatusCanceled is the status when a job is canceled
	JobStatusCanceled

	// JobStatusFailed is the status when a job has failed and would not succeed even if retried
	JobStatusFailed

	// JobStatusSucceeded is the status when a job has completed successfully
	JobStatusSucceeded
)

var jobStatuses = map[string]jobStatusEnum{
	"PENDING":   JobStatusPending,
	"ERRORED":   JobStatusErrored,
	"CANCELED":  JobStatusCanceled,
	"FAILED":    JobStatusFailed,
	"SUCCEEDED": JobStatusSucceeded,
}

func (j jobStatusEnum) String() string {
	switch j {
	case JobStatusPending:
		return "PENDING"
	case JobStatusErrored:
		return "ERRORED"
	case JobStatusCanceled:
		return "CANCELED"
	case JobStatusFailed:
		return "FAILED"
	case JobStatusSucceeded:
		return "SUCCEEDED"
	default:
		return ""
	}
}

// Scan implements the Scanner interface
func (j *jobStatusEnum) Scan(value interface{}) error {
	s, _ := value.(string)
	s = strings.ToUpper(s)

	if v, ok := jobStatuses[s]; ok {
		*j = v
		return nil
	}

	return ErrInvalidJobStatus
}

// Value implements the Valuer interface
func (j jobStatusEnum) Value() (driver.Value, error) {
	v := j.String()
	if v != "" {
		return driver.Value(v), nil
	}

	return nil, ErrInvalidJobStatus
}

var (
	_ driver.Valuer = (*jobStatusEnum)(nil)
	_ sql.Scanner   = (*jobStatusEnum)(nil)
)

// FormatSlackActionID creates a pipe ("|") separated Slack action ID
// string consisting of the job type as the key and an arbitrary value
// to uniquely identify an interactive Slack component.
func FormatSlackActionID(key jobTypeEnum, value interface{}) string {
	return fmt.Sprintf("%s|%v", key.String(), value)
}

// ExtractJobFromActionID extracts the job type from a Slack action ID
func ExtractJobFromActionID(actionID string) (jobTypeEnum, error) {
	parts := strings.Split(actionID, "|")

	if len(parts) == 2 {
		job := parts[0]

		if v, ok := jobTypes[job]; ok {
			return v, nil
		}
	}

	return 0, ErrInvalidJobType
}

// JobRequiresSlackChannel ...
func JobRequiresSlackChannel(jobType jobTypeEnum) bool {
	switch jobType {
	case JobTypeSyncChannels, JobTypeGreetAdmin, JobTypeAddChannel, JobTypeUpdateMatch:
		return false
	default:
		return true
	}
}
