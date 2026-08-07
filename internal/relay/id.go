package relay

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type idGenerator func() (string, error)

func secureID(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate secure ID: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
