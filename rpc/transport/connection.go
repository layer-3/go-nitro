package transport

type ConnectionType string

const (
	Nats ConnectionType = "nats"
	Ws   ConnectionType = "ws"
)

// Requester is a connection that can send requests and subscribe to notifications
type Requester interface {
	// Close closes the connection
	Close()

	// Request sends data for a topic and returns the response data or an error
	Request(uint64, []byte) ([]byte, error)
	// Subscribe listens for notifications for a topic and does not respond to notifications
	Subscribe() (<-chan []byte, error)
}

// Responder is a connection that can respond to requests and send notifications
type Responder interface {
	// Close closes the connection
	Close()
	// Url returns the url that the responder is listening on
	Url() string

	// Respond listens for requests for a topic and calls the handler function when a request is received
	Respond(func([]byte) []byte) error
	// Notify sends data for a topic without expecting a response
	Notify([]byte) error
}