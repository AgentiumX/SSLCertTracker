package alert

import "context"

type EmailChannel struct {
	config string
}

func (c *EmailChannel) Send(ctx context.Context, msg Message) error {
	return nil // TODO: implement in task 7
}

func (c *EmailChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *EmailChannel) ValidateConfig() error {
	return nil // TODO: implement in task 7
}
