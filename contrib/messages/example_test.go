package messages_test

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow/contrib/messages"
)

func ExampleInject() {
	ctx := messages.Inject(context.Background(), []messages.Message{
		{Level: messages.Success, Text: "Item created"},
	})

	msgs := messages.Get(ctx)
	fmt.Println(msgs[0].Level, msgs[0].Text)
	// Output:
	// success Item created
}

func ExampleGet() {
	// Without messages in the context, Get returns nil.
	fmt.Println(messages.Get(context.Background()))
	// Output:
	// []
}

func ExampleMessage() {
	msg := messages.Message{Level: messages.Warning, Text: "Disk space low"}
	fmt.Println(msg.Level)
	fmt.Println(msg.Text)
	// Output:
	// warning
	// Disk space low
}
