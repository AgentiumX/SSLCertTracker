package alert

type EmailChannel struct {
	config string
}

func (c *EmailChannel) Send(msg Message) error {
	return nil // TODO: implement in task 7
}

func (c *EmailChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *EmailChannel) ValidateConfig() error {
	return nil // TODO: implement in task 7
}
