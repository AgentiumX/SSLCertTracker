package alert

import "context"

type FeishuChannel struct {
	config string
}

func (c *FeishuChannel) Send(ctx context.Context, msg Message) error {
	return nil // TODO: implement in task 5
}

func (c *FeishuChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *FeishuChannel) ValidateConfig() error {
	return nil // TODO: implement in task 5
}
