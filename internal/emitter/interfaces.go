package emitter

type MessagePublisher interface {
	Publish(msg string)
	GetMessage() (string, error)
	Close()
}
