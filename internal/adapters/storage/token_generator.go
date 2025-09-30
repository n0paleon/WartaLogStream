package storage

import (
	"crypto/rand"
	"fmt"
)

func GenerateToken() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
