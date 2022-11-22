package bot

import (
	"time"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/timex"
)

// FirstChatRouletteRound returns the timestamp of the first chat roulette round.
func FirstChatRouletteRound(t time.Time, weekday string, hour int) time.Time {
	return timex.NextWeekday(t, weekday, hour)
}

// NextChatRouletteRound returns the timestamp of the next chat roulette round.
func NextChatRouletteRound(t time.Time, interval models.IntervalEnum) time.Time {
	return t.AddDate(0, 0, int(interval))
}
