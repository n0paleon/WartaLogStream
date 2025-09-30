package ports

import (
	"sync"
	"time"
)

type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]SessionStorage
	ticker   *time.Ticker
}

// Buat registry baru
func NewSessionRegistry() *SessionRegistry {
	r := &SessionRegistry{
		sessions: make(map[string]SessionStorage),
		ticker:   time.NewTicker(time.Minute), // cek expired tiap menit
	}

	// goroutine untuk auto-expire
	go func() {
		for range r.ticker.C {
			r.cleanupExpired()
		}
	}()

	return r
}

// Tambah session baru
func (r *SessionRegistry) Add(session SessionStorage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.GetID()] = session
}

// Ambil session
func (r *SessionRegistry) Get(id string) (SessionStorage, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sess, ok := r.sessions[id]
	return sess, ok
}

// Hapus session
func (r *SessionRegistry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[id]; ok {
		delete(r.sessions, id)
	}
}

// Bersihkan session yang expired
func (r *SessionRegistry) cleanupExpired() {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, sess := range r.sessions {
		if sess.GetExpiredAt().Before(now) {
			delete(r.sessions, id)
		}
	}
}

// Dapatkan semua session aktif (opsional)
func (r *SessionRegistry) List() []SessionStorage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]SessionStorage, 0, len(r.sessions))
	for _, s := range r.sessions {
		res = append(res, s)
	}
	return res
}
