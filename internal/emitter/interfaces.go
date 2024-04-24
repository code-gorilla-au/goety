package emitter

type MessagePublisher interface {
	Publish(msg string)
}
