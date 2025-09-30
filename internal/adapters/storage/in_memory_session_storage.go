package storage

import (
	"WartaLogStream/internal/adapters/utils"
	"WartaLogStream/internal/ports"
	"errors"
	"github.com/oklog/ulid/v2"
	"strconv"
	"sync"
	"time"
)

var HeartBeatTimeout = 1 * time.Minute

type InMemorySessionStorage struct {
	id             string
	token          string
	maxEntries     int
	logs           [][]byte
	noteLines      []string
	mu             sync.Mutex
	stopped        bool
	createdAt      time.Time
	stoppedAt      time.Time
	expiredAt      time.Time
	subscribers    map[chan ports.Event]struct{}
	lastWriterPing time.Time
}

// Constructor
func NewInMemoryStorage(maxEntries int) *InMemorySessionStorage {
	session := &InMemorySessionStorage{
		id:             ulid.Make().String(),
		maxEntries:     maxEntries,
		logs:           make([][]byte, 0, maxEntries),
		noteLines:      make([]string, 0, 10),
		createdAt:      time.Now(),
		expiredAt:      time.Now().Add(24 * time.Hour),
		subscribers:    make(map[chan ports.Event]struct{}),
		lastWriterPing: time.Now(),
	}

	session.startWriterHeartbeat()

	return session
}

func (s *InMemorySessionStorage) ValidateToken(token string) error {
	if token != s.token {
		return errors.New("invalid token")
	}
	return nil
}

func (s *InMemorySessionStorage) startWriterHeartbeat() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.mu.Lock()
			if s.stopped {
				s.mu.Unlock()
				return
			}
			if time.Since(s.lastWriterPing) > HeartBeatTimeout {
				s.mu.Unlock() // required, do not remove
				_ = s.Stop(s.token)
				return
			}
			s.mu.Unlock()
		}
	}()
}

// stopInternal warning, this method is not using mutex to call, make sure the caller set mutex lock first before calling this method
func (s *InMemorySessionStorage) stopInternal() {
	if s.stopped {
		return
	}
	s.stopped = true
	s.stoppedAt = time.Now()
	s.broadcast(ports.Event{Type: ports.EventStatus, Data: "Stopped"})
}

// UpdateWriterPing digunakan writer untuk menandai aktivitas.
// Harus dipanggil oleh writer setiap kali ada aksi (PushLog, SetNote, dsb).
func (s *InMemorySessionStorage) UpdateWriterPing() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastWriterPing = time.Now()
}

func (s *InMemorySessionStorage) SetToken(token string) {
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
}

func (s *InMemorySessionStorage) GetID() string {
	return s.id
}

func (s *InMemorySessionStorage) GetExpiredAt() time.Time {
	return s.expiredAt
}

func (s *InMemorySessionStorage) Stop(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return errors.New("session stopped")
	}
	if err := s.ValidateToken(token); err != nil {
		return err
	}

	s.stopped = true
	s.stoppedAt = time.Now()
	s.broadcast(ports.Event{Type: ports.EventStatus, Data: "Stopped"})

	return nil
}

// PushLog
func (s *InMemorySessionStorage) PushLog(token string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return errors.New("session stopped")
	}

	if err := s.ValidateToken(token); err != nil {
		return err
	}

	compressed, err := utils.Compress(data)
	if err != nil {
		return err
	}

	if len(s.logs) == s.maxEntries {
		s.logs = s.logs[1:]
	}
	s.logs = append(s.logs, compressed)

	// broadcast log
	s.broadcast(ports.Event{Type: ports.EventLog, Data: data})
	return nil
}

// SetNote
func (s *InMemorySessionStorage) SetNote(token string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return errors.New("session stopped")
	}

	if err := s.ValidateToken(token); err != nil {
		return err
	}

	// potong panjang baris
	if len(data) > 500 {
		data = data[:500]
	}

	// simpan di noteLines (max 10)
	if len(s.noteLines) == 10 {
		s.noteLines = s.noteLines[1:]
	}
	s.noteLines = append(s.noteLines, data)

	// broadcast note
	s.broadcast(ports.Event{Type: ports.EventNote, Data: data})
	return nil
}

// broadcast ke semua subscriber
func (s *InMemorySessionStorage) broadcast(evt ports.Event) {
	for ch := range s.subscribers {
		select {
		case ch <- evt:
		default:
			// skip kalau channel penuh
		}
	}
}

// SubscribeLog sekarang return domain.Event channel
func (s *InMemorySessionStorage) SubscribeLog() chan ports.Event {
	ch := make(chan ports.Event, 100)

	// ambil snapshot log dan note
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[ch] = struct{}{}

	logsSnapshot := make([]string, 0, len(s.logs))
	for _, log := range s.logs {
		d, err := utils.Decompress(log)
		if err == nil {
			logsSnapshot = append(logsSnapshot, d)
		}
	}

	notesSnapshot := append([]string{}, s.noteLines...) // copy slice

	go func() {
		ch <- ports.Event{Type: ports.EventSessionCreationTime, Data: strconv.Itoa(int(s.createdAt.Unix()))}

		for _, d := range logsSnapshot {
			select {
			case ch <- ports.Event{Type: ports.EventLog, Data: d}:
			default:
			}
		}
		for _, line := range notesSnapshot {
			select {
			case ch <- ports.Event{Type: ports.EventNote, Data: line}:
			default:
			}
		}

		status := "Running"
		if s.stopped {
			status = "Stopped"
			ch <- ports.Event{Type: ports.EventSessionStopTime, Data: strconv.Itoa(int(s.stoppedAt.Unix()))}
		}
		ch <- ports.Event{Type: ports.EventStatus, Data: status}
	}()

	return ch
}

// Unsubscribe
func (s *InMemorySessionStorage) UnsubscribeLog(ch chan ports.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subscribers[ch]; ok {
		delete(s.subscribers, ch)
		close(ch)
	}
}
