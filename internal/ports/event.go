package ports

type EventType string

const (
	EventLog                 EventType = "log"
	EventNote                EventType = "note"
	EventStatus              EventType = "status"
	EventSessionCreationTime EventType = "session_creation_time"
	EventSessionStopTime     EventType = "session_stop_time"
)

type Event struct {
	Type EventType `json:"type"`
	Data string    `json:"data"`
}
