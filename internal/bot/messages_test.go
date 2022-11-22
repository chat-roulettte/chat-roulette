package bot

import (
	"encoding/json"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
)

func Test_transformMessage(t *testing.T) {
	raw := []byte(`
{
    "blocks":[{
        "type":"section",
        "text":{
            "type":"mrkdwn",
            "text":"Hello <@U0123456789> :wave:\n\nWelcome to the <#G0123456789> channel!",
            "verbatim":false
        }
    },
    {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": "Please click the button below if you wish to participate in chat roulette"
        }
    },
    {
        "type": "actions",
        "elements": [
            {
                "action_id": "TEST",
                "type": "button",
                "text": {
                    "type": "plain_text",
                    "emoji": true,
                    "text": ":white_check_mark: Confirm"
                },
                "style": "primary",
                "value": "true"
            }
        ]
    }
    ]
}`)

	var originalMessage slack.Message
	err := json.Unmarshal(raw, &originalMessage)
	assert.Nil(t, err)

	text := slack.NewTextBlockObject("mrkdwn", "Hello World", false, false)
	section := slack.NewSectionBlock(text, nil, nil)

	newMessage := transformMessage(originalMessage, 1, section)
	assert.Len(t, newMessage.Blocks.BlockSet, 2)

	b, err := newMessage.Blocks.MarshalJSON()
	assert.Nil(t, err)

	assert.Contains(t, string(b), `"type":"mrkdwn","text":"Hello World"`)
}
