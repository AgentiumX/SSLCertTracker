package idgen

import (
	"crypto/sha256"
	"fmt"
	"os"
	"time"
)

func GenerateID(hostname, ip string) string {
	input := fmt.Sprintf("%s%s%d", hostname, ip, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:8])
}

func LoadOrCreateID(idFilePath, hostname, ip string) (string, error) {
	data, err := os.ReadFile(idFilePath)
	if err == nil && len(data) == 16 {
		return string(data), nil
	}
	id := GenerateID(hostname, ip)
	if err := os.WriteFile(idFilePath, []byte(id), 0600); err != nil {
		return "", err
	}
	return id, nil
}
