package emitter

type MessagePublisher interface {
	Publish(msg string)
}

type MessageGetPublish interface {
	MessagePublisher
	GetMessage() (string, error)
}

type MessageGetPublishCloser interface {
	MessagePublisher
	MessageGetPublish
	Close()
}
