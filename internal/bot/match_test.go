package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PairParticipants(t *testing.T) {
	members := []string{
		"bob",
		"alice",
		"ahmed",
		"musa",
		"sarah",
		"jordan",
		"kakarot",
		"yami",
		"curtis",
		"jared",
	}

	var matches []ChatRoulettePair
	for _, i := range members {
		for _, j := range members {
			if i == j {
				continue
			}

			matches = append(matches, ChatRoulettePair{
				Participant: i,
				Partner:     j,
			})
		}
	}

	actual := PairParticipants(matches)

	expected := map[string]string{
		"bob":     "alice",
		"ahmed":   "musa",
		"sarah":   "jordan",
		"kakarot": "yami",
		"curtis":  "jared",
	}

	assert.Equal(t, expected, actual)
	assert.Len(t, actual, 5)
}
