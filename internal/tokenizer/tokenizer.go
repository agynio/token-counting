package tokenizer

import (
	"fmt"
	"unicode/utf8"
)

type Model string

const ModelGPT5 Model = "gpt-5"

type Message struct {
	payload []byte
}

func NewMessage(payload []byte) Message {
	return Message{payload: payload}
}

func CountTokens(model Model, messages []Message) []int32 {
	if model != ModelGPT5 {
		panic(fmt.Sprintf("unsupported model: %s", model))
	}

	tokens := make([]int32, len(messages))
	for i, message := range messages {
		count := utf8.RuneCount(message.payload)
		tokens[i] = int32((count + 3) / 4)
	}
	return tokens
}
