package alert

type FeishuChannel struct {
	config string
}

func (c *FeishuChannel) Send(msg Message) error {
	return nil // TODO: implement in task 5
}

func (c *FeishuChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *FeishuChannel) ValidateConfig() error {
	return nil // TODO: implement in task 5
}
