package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

func TestNextChatRouletteRound(t *testing.T) {
	now := time.Date(2021, time.March, 1, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		interval models.IntervalEnum
		expected time.Time
	}{
		{models.Weekly, time.Date(2021, time.March, 8, 12, 0, 0, 0, time.UTC)},
		{models.Biweekly, time.Date(2021, time.March, 15, 12, 0, 0, 0, time.UTC)},
		{models.Triweekly, time.Date(2021, time.March, 22, 12, 0, 0, 0, time.UTC)},
		{models.Quadweekly, time.Date(2021, time.March, 29, 12, 0, 0, 0, time.UTC)},
		{models.Monthly, time.Date(2021, time.April, 5, 12, 0, 0, 0, time.UTC)},
	}

	for _, tt := range testCases {
		t.Run(tt.interval.String(), func(t *testing.T) {
			actual := NextChatRouletteRound(now, tt.interval)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestFormatSchedule(t *testing.T) {
	testCases := []struct {
		name      string
		interval  models.IntervalEnum
		timestamp time.Time
		expected  string
	}{
		{
			name:      "Weekly on Mondays",
			interval:  models.Weekly,
			timestamp: time.Date(2024, 8, 5, 0, 0, 0, 0, time.UTC),
			expected:  "*Weekly* on *Mondays*",
		},
		{
			name:      "Biweekly on Tuesdays",
			interval:  models.Biweekly,
			timestamp: time.Date(2024, 8, 6, 0, 0, 0, 0, time.UTC),
			expected:  "*Biweekly* on *Tuesdays*",
		},
		{
			name:      "Monthly",
			interval:  models.Monthly,
			timestamp: time.Date(2024, 8, 2, 0, 0, 0, 0, time.UTC),
			expected:  "On the *first Friday* of every month",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			actual := formatSchedule(tt.interval, tt.timestamp)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
