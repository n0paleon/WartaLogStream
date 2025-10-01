package ports

import (
	"time"
)

type SessionStorage interface {
	GetID() string
	PushLog(data string) error
	SetNote(data string) error
	ValidateToken(token string) error
	Stop() error
	GetExpiredAt() time.Time
	SubscribeLog() chan Event
	UnsubscribeLog(ch chan Event)
	UpdateWriterPing()
}
