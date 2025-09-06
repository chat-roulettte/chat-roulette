package bot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/go-playground/tz"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

type privateMetadata struct {
	ChannelID   string       `json:"channel_id,omitempty"`
	ResponseURL string       `json:"response_url,omitempty"`
	Blocks      slack.Blocks `json:"blocks,omitempty"`
}

// Encode encodes privateMetadata from struct to json to base64
func (p *privateMetadata) Encode() (string, error) {
	var b bytes.Buffer

	encoder := base64.NewEncoder(base64.StdEncoding, &b)
	if err := json.NewEncoder(encoder).Encode(p); err != nil {
		return "", err
	}
	encoder.Close()

	return b.String(), nil
}

// Decode decodes privateMetadata from base64 to json to struct
func (p *privateMetadata) Decode(s string) error {
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(s))
	return json.NewDecoder(decoder).Decode(p)
}

func ExtractChannelIDFromPrivateMetada(interaction *slack.InteractionCallback) (string, error) {
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return "", errors.Wrap(err, "failed to extract channel ID from privateMetadata")
	}

	return pm.ChannelID, nil
}

// onboardingTemplate is used with templates/onboarding_*.json.tmpl templates
type onboardingTemplate struct {
	UserID          string
	ChannelID       string
	PrivateMetadata string
	ImageURL        string
	Zones           []tz.Zone
	IsAdmin         bool
	ConnectionMode  string
}
