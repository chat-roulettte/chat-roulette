package timex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NextWeekday(t *testing.T) {
	// Testing against Wednesday, Jan 13th, 2021 12:00 UTC
	now := time.Date(2021, time.January, 13, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		weekday  string
		hour     int
		expected time.Time
	}{
		{"next Friday", "Friday", 12, time.Date(2021, time.January, 15, 12, 0, 0, 0, time.UTC)},
		{"next Saturday", "Saturday", 15, time.Date(2021, time.January, 16, 15, 0, 0, 0, time.UTC)},
		{"next Monday", "Monday", 20, time.Date(2021, time.January, 18, 20, 0, 0, 0, time.UTC)},
		{"next Tuesday", "Tuesday", 9, time.Date(2021, time.January, 19, 9, 0, 0, 0, time.UTC)},
		{"next Wednesday", "Wednesday", 9, time.Date(2021, time.January, 20, 9, 0, 0, 0, time.UTC)},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			actual := NextWeekday(now, tt.weekday, tt.hour)
			assert.Equal(t, tt.expected, actual)
		})
	}

	// Verify the new date is in the new year
	t.Run("new year next Monday", func(t *testing.T) {
		// Wednesday, Dec 30th, 2020 12:00 UTC
		now := time.Date(2020, time.December, 30, 12, 0, 0, 0, time.UTC)

		actual := NextWeekday(now, "Monday", 9)
		expected := time.Date(2021, time.January, 4, 9, 0, 0, 0, time.UTC)

		assert.Equal(t, expected, actual)
	})

	// Verify the new date is 2 weeks in the future from next Monday
	t.Run("2 weeks", func(t *testing.T) {
		// Monday, Jan 4th, 2021 12:00 UTC
		now := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

		actual := NextWeekday(now, "Monday", 9).AddDate(0, 0, 14) // 2 weeks
		expected := time.Date(2021, time.January, 25, 9, 0, 0, 0, time.UTC)

		assert.Equal(t, expected, actual)
	})

	// Verify the new date is 4 weeks in the future from next Monday
	t.Run("4 weeks", func(t *testing.T) {
		// Monday, Jan 4th, 2021 12:00 UTC
		now := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

		actual := NextWeekday(now, "Monday", 9).AddDate(0, 0, 28) // 4 weeks
		expected := time.Date(2021, time.February, 8, 9, 0, 0, 0, time.UTC)

		assert.Equal(t, expected, actual)
	})
}
