package alert

import "context"

type WebhookChannel struct {
	config string
}

func (c *WebhookChannel) Send(ctx context.Context, msg Message) error {
	return nil // TODO: implement in task 3
}

func (c *WebhookChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *WebhookChannel) ValidateConfig() error {
	return nil // TODO: implement in task 3
}
