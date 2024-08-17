package timex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseWeekday(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Weekday
		isErr    bool
	}{
		{"Mon", time.Monday, false},
		{"Tue", time.Tuesday, false},
		{"Tues", time.Tuesday, false},
		{"Thu", time.Thursday, false},
		{"Thurs", time.Thursday, false},
		{"Wed", time.Wednesday, false},
		{"InvalidDay", time.Sunday, true},
		{"F", time.Friday, true},
		{"S", time.Saturday, true},
		{"Satu", time.Saturday, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual, err := ParseWeekday(tt.input)
			if tt.isErr {
				// assert.Equal(t, tt.err, err, "expected error %v, got %v", tt.err, err)
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}

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

func TestNextMonth(t *testing.T) {
	testCases := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "first Monday",
			input:    time.Date(2024, time.June, 4, 11, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.July, 2, 11, 0, 0, 0, time.UTC),
		},
		{
			name:     "second Tuesday",
			input:    time.Date(2024, time.June, 11, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.July, 9, 10, 0, 0, 0, time.UTC),
		},
		{
			name:     "third Friday",
			input:    time.Date(2023, time.September, 16, 8, 0, 0, 0, time.UTC),
			expected: time.Date(2023, time.October, 21, 8, 0, 0, 0, time.UTC),
		},
		{
			name:     "last Saturday",
			input:    time.Date(2022, time.May, 28, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2022, time.June, 25, 10, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of year rollover",
			input:    time.Date(2023, time.December, 28, 15, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.January, 25, 15, 0, 0, 0, time.UTC),
		},
		{
			name:     "current month has 5 weeks",
			input:    time.Date(2022, time.May, 2, 3, 0, 0, 0, time.UTC),
			expected: time.Date(2022, time.June, 6, 3, 0, 0, 0, time.UTC),
		},
		{
			name:     "new month has 5 weeks",
			input:    time.Date(2022, time.March, 26, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2022, time.April, 30, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "both months have 5 weeks",
			input:    time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.February, 5, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year",
			input:    time.Date(2024, time.January, 25, 7, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.February, 29, 7, 0, 0, 0, time.UTC), // Example date
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			actual := NextMonth(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestFormatMonthlyInterval(t *testing.T) {
	testCases := []struct {
		input    time.Time
		expected string
	}{
		{time.Date(2024, time.August, 1, 12, 0, 0, 0, time.UTC), "first Thursday"},
		{time.Date(2024, time.August, 12, 12, 0, 0, 0, time.UTC), "second Monday"},
		{time.Date(2024, time.August, 22, 12, 0, 0, 0, time.UTC), "last Thursday"},
		{time.Date(2024, time.August, 21, 12, 0, 0, 0, time.UTC), "third Wednesday"},
		{time.Date(2024, time.August, 30, 12, 0, 0, 0, time.UTC), "last Friday"},
	}

	for _, tt := range testCases {
		actual := FormatMonthlyOccurrence(tt.input)
		assert.Equal(t, tt.expected, actual)
	}
}
