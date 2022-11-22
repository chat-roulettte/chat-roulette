package templatex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

func Test_Capitalize(t *testing.T) {
	s := "friday"

	actual := Capitalize(s)
	expected := "Friday"

	assert.Equal(t, expected, actual)
}

func Test_CapitalizeInterval(t *testing.T) {
	actual := CapitalizeInterval(models.Biweekly)
	expected := "Biweekly"

	assert.Equal(t, expected, actual)
}

func Test_PrettyDate(t *testing.T) {
	date := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

	actual := PrettyDate(date)
	expected := "January 4th, 2021"

	assert.Equal(t, expected, actual)
}

func Test_PrettierDate(t *testing.T) {
	date := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

	actual := PrettierDate(date)
	expected := "Monday, January 4th, 2021"

	assert.Equal(t, expected, actual)
}

func Test_DerefBool(t *testing.T) {
	b := true

	actual := DerefBool(&b)
	expected := true

	assert.Equal(t, expected, actual)
}
