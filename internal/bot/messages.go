package bot

import "github.com/slack-go/slack"

// slackMessage is a generic struct to unmarshal a template into a Slack message that contains Blocks and Attachments.
type slackMessage struct {
	Blocks      slack.Blocks       `json:"blocks"`
	Attachments []slack.Attachment `json:"attachments"`
}

// transformMessage transforms a slack.Message by preserving the first N blocks and appending new blocks.
func transformMessage(message slack.Message, n int, blocks ...slack.Block) slack.Message {
	preserved := min(n, len(message.Blocks.BlockSet))

	msg := slack.Message{
		Msg: slack.Msg{
			Blocks: slack.Blocks{
				BlockSet: make([]slack.Block, 0, preserved+len(blocks)),
			},
		},
	}

	msg.Blocks.BlockSet = append(msg.Blocks.BlockSet, message.Blocks.BlockSet[:preserved]...)
	msg.Blocks.BlockSet = append(msg.Blocks.BlockSet, blocks...)

	return msg
}
