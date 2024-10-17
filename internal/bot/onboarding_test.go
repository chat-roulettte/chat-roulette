package bot

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
)

func Test_privateMetadata_Encode(t *testing.T) {
	pm := privateMetadata{
		ChannelID:   "C1234567890",
		ResponseURL: "https://example.com/example",
		Blocks:      slack.Blocks{},
	}

	encoded, err := pm.Encode()
	assert.NoError(t, err)
	assert.NotEmpty(t, encoded)
	assert.Equal(t, encoded, "eyJjaGFubmVsX2lkIjoiQzEyMzQ1Njc4OTAiLCJyZXNwb25zZV91cmwiOiJodHRwczovL2V4YW1wbGUuY29tL2V4YW1wbGUiLCJibG9ja3MiOm51bGx9Cg==")
}

func Test_privateMetadata_Decode(t *testing.T) {
	encoded := "eyJjaGFubmVsX2lkIjoiQzEyMzQ1Njc4OTAiLCJyZXNwb25zZV91cmwiOiJodHRwczovL2V4YW1wbGUuY29tL2V4YW1wbGUiLCJibG9ja3MiOm51bGx9Cg=="

	var pm privateMetadata
	err := pm.Decode(encoded)
	assert.NoError(t, err)
	assert.Equal(t, "C1234567890", pm.ChannelID)
	assert.Equal(t, "https://example.com/example", pm.ResponseURL)
}

func Test_ExtractChannelIDFromPrivateMetada(t *testing.T) {
	pm := privateMetadata{
		ChannelID:   "C1234567890",
		ResponseURL: "https://example.com/foo",
	}

	encoded, err := pm.Encode()
	assert.NoError(t, err)

	interaction := &slack.InteractionCallback{
		View: slack.View{
			PrivateMetadata: encoded,
		},
	}

	channelID, err := ExtractChannelIDFromPrivateMetada(interaction)
	assert.NoError(t, err)
	assert.Equal(t, pm.ChannelID, channelID)
}

func Test_ExtractChannelIDFromPrivateMetada_Invalid(t *testing.T) {
	interaction := &slack.InteractionCallback{
		View: slack.View{
			PrivateMetadata: "not-valid-base64-encoded-string",
		},
	}

	channelID, err := ExtractChannelIDFromPrivateMetada(interaction)
	assert.Error(t, err)
	assert.Empty(t, channelID)
}
