package bot

import "github.com/slack-go/slack"

// transformMessage transforms a slack.Message by preserving
// the first N blocks and appending a new block.
func transformMessage(message slack.Message, count int, block slack.Block) slack.Message {
	var newMessage slack.Message

	for i := 0; i < count; i++ {
		b := message.Blocks.BlockSet[i]

		newMessage.Blocks.BlockSet = append(newMessage.Blocks.BlockSet, b)
	}

	return slack.AddBlockMessage(newMessage, block)
}
