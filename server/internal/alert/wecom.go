package alert

type WecomChannel struct {
	config string
}

func (c *WecomChannel) Send(msg Message) error {
	return nil // TODO: implement in task 6
}

func (c *WecomChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *WecomChannel) ValidateConfig() error {
	return nil // TODO: implement in task 6
}
