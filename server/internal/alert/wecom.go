package alert

import "context"

type WecomChannel struct {
	config string
}

func (c *WecomChannel) Send(ctx context.Context, msg Message) error {
	return nil // TODO: implement in task 6
}

func (c *WecomChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *WecomChannel) ValidateConfig() error {
	return nil // TODO: implement in task 6
}
