package alert

type WebhookChannel struct {
	config string
}

func (c *WebhookChannel) Send(msg Message) error {
	return nil // TODO: implement in task 3
}

func (c *WebhookChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *WebhookChannel) ValidateConfig() error {
	return nil // TODO: implement in task 3
}
