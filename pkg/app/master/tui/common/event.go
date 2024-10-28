package common

const (
	GetImagesEvent   EventType = "getImages"
	LaunchDebugEvent EventType = "launchDebug"
)

type (
	// EventType identifies the type of event
	EventType string
	// Event represents an event in the lifecycle of a resource
	Event struct {
		Type EventType
		Data interface{}
	}
)
