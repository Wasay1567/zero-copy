package helpers

import (
	"crypto/rand"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateRandomString produces a lightweight 8-character string
func generateRandomString() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	return string(bytes), nil
}

func BuildNameForHostVeth(containerid string) (string, error) {
	if len(containerid)<=8 {
		r, err := generateRandomString()
		if err != nil {
			return "", err
		}
		hostVethName := "veth-" + r
		return hostVethName, nil
	}
	
	hostVethName := "veth-" + containerid[:8] // Unique host-side name
	return hostVethName, nil
}