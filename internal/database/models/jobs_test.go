package models

import (
	"errors"
	"testing"

	"github.com/dmarkham/enumer/enumerrs"
	"github.com/stretchr/testify/assert"
)

func Test_FormatSlackActionID(t *testing.T) {
	expected := "CHECK_PAIR|true"
	actual := FormatSlackActionID(JobTypeCheckPair, true)

	assert.Equal(t, expected, actual)
}

func Test_ExtractJobFromActionID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		actionID := "GREET_MEMBER|false"

		jobType, err := ExtractJobFromActionID(actionID)

		assert.Nil(t, err)
		assert.Equal(t, JobTypeGreetMember, jobType)
	})

	t.Run("malformed", func(t *testing.T) {
		actionID := "qwerty"

		_, err := ExtractJobFromActionID(actionID)
		assert.NotNil(t, err)
	})
}

func Test_JobRequiresSlackChannel(t *testing.T) {
	t.Run("ADD_CHANNEL", func(t *testing.T) {
		v := JobRequiresSlackChannel(JobTypeAddChannel)
		assert.False(t, v)
	})

	t.Run("ADD_MEMBER", func(t *testing.T) {
		v := JobRequiresSlackChannel(JobTypeAddMember)
		assert.True(t, v)
	})
}

func Test_IsError(t *testing.T) {
	v, err := jobTypeEnumString("FOO_BAR")
	assert.Error(t, err)
	assert.Zero(t, v)
	assert.True(t, errors.Is(err, enumerrs.ErrValueInvalid))
	assert.Contains(t, err.Error(), "does not belong to")
}
