package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dmarkham/enumer/enumerrs"
)

var (
	ErrJobParamsFailedValidation = errors.New("failed to validate job parameters")
)

const (
	// JobPriority are the priorities to use for jobs
	JobPriorityHighest  = 10
	JobPriorityHigh     = 8
	JobPriorityStandard = 5
	JobPriorityLow      = 3
	JobPriorityLowest   = 1
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

		return jobTypeEnumString(job)
	}

	return 0, enumerrs.ErrValueInvalid
}

// JobRequiresSlackChannel ...
func JobRequiresSlackChannel(jobType jobTypeEnum) bool {
	switch jobType {
	case JobTypeSyncChannels, JobTypeAddChannel:
		return false
	case JobTypeGreetAdmin, JobTypeUpdateMatch:
		return false
	case JobTypeBlockMember, JobTypeUnblockMember:
		return false
	default:
		return true
	}
}
