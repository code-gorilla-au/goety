package emitter

// MessagePublisher is an interface that defines the method to publish a message
type MessagePublisher interface {
	Publish(msg string)
}

// MessageGetPublish is an interface that defines the method to publish a message and get a message
type MessageGetPublish interface {
	MessagePublisher
	GetMessage() (string, error)
}

// MessageGetPublishCloser is an interface that defines the method to publish a message, get a message and close the emitter
type MessageGetPublishCloser interface {
	MessagePublisher
	MessageGetPublish
	Close()
}
