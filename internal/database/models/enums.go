package models

// jobStatusEnum is an enum for the completion status for jobs
//
//go:generate enumer -type=jobStatusEnum -trimprefix=JobStatus -text -sql -json -typederrors -transform=upper -output=generated_job_status.go
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

// jobTypeEnum is an enum for the various jobs in the queue
//
//go:generate enumer -type=jobTypeEnum -trimprefix=JobType -text -sql -json -typederrors -transform=snake-upper -output=generated_job_type.go
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

	// JobTypeBlockMember is the job for blocking a Slack member from being matched with a user
	JobTypeBlockMember

	// JobTypeUnblockMember is the job for unblocking a Slack member from being matched with a user
	JobTypeUnblockMember
)

// IntervalEnum is an enum for chat roulette intervals
//
//go:generate enumer -type=IntervalEnum -text -sql -json -typederrors -transform=lower -output=generated_interval.go
type IntervalEnum int64

const (
	// Weekly is every 7 days
	Weekly IntervalEnum = 7

	// Biweekly is every 2 weeks, 14 days
	Biweekly IntervalEnum = 14

	// Triweekly is every 3 weeks, 21 days
	Triweekly IntervalEnum = 21

	// Quadweekly is every 4 weeks, 28 days
	Quadweekly IntervalEnum = 28

	// Monthly is every month on the same week
	Monthly IntervalEnum = 30
)

//go:generate enumer -type=Gender -text -json -sql -typederrors -transform=lower -output=gender_generated.go
type Gender int8

const (
	// Male represents the male gender
	Male Gender = iota + 1

	// Female represents the female gender
	Female
)

// ConnectionMode is an enum for connection modes
//
//go:generate enumer -type=ConnectionMode -text -json -sql -typederrors -trimprefix=ConnectionMode -transform=lower -output=generated_connection_mode.go
type ConnectionMode int64

const (
	// Virtual represents a virtual connection over Zoom, Meet, etc.
	ConnectionModeVirtual ConnectionMode = iota + 1

	// Physical represents a physical connection in the real world
	ConnectionModePhysical

	// Hybrid represents both a virtual or physical connection
	ConnectionModeHybrid
)
