package bot

import (
	"fmt"
	"time"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/templatex"
	"github.com/chat-roulettte/chat-roulette/internal/timex"
)

// FirstChatRouletteRound returns the timestamp of the first chat roulette round.
func FirstChatRouletteRound(t time.Time, weekday string, hour int) time.Time {
	return timex.NextWeekday(t, weekday, hour)
}

// NextChatRouletteRound returns the timestamp of the next chat roulette round.
func NextChatRouletteRound(t time.Time, interval models.IntervalEnum) time.Time {
	var timestamp time.Time

	switch interval {
	case models.Monthly:
		timestamp = timex.NextMonth(t)
	default:
		timestamp = t.AddDate(0, 0, int(interval))
	}

	return timestamp
}

// formatSchedule ...
func formatSchedule(interval models.IntervalEnum, t time.Time) string {
	when := fmt.Sprintf("*%s* on *%ss*", templatex.Capitalize(interval.String()), t.Weekday())

	if interval == models.Monthly {
		when = fmt.Sprintf("On the *%s* of every month", timex.FormatMonthlyOccurrence(t))
	}

	return when
}
