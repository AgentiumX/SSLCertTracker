package alert

type DingtalkChannel struct {
	config string
}

func (c *DingtalkChannel) Send(msg Message) error {
	return nil // TODO: implement in task 4
}

func (c *DingtalkChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *DingtalkChannel) ValidateConfig() error {
	return nil // TODO: implement in task 4
}
