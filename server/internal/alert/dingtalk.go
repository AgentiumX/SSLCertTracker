package alert

import "context"

type DingtalkChannel struct {
	config string
}

func (c *DingtalkChannel) Send(ctx context.Context, msg Message) error {
	return nil // TODO: implement in task 4
}

func (c *DingtalkChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *DingtalkChannel) ValidateConfig() error {
	return nil // TODO: implement in task 4
}
