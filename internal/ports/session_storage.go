package ports

import (
	"time"
)

type SessionStorage interface {
	GetID() string
	PushLog(token string, data string) error
	SetNote(token string, data string) error
	Stop(token string) error
	GetExpiredAt() time.Time
	SubscribeLog() chan Event
	UnsubscribeLog(ch chan Event)
	UpdateWriterPing()
}
