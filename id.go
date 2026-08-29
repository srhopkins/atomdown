package atomdown

import (
	"crypto/rand"
	"fmt"
)

const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewID returns an eight-character Crockford Base32 identifier with 40 random bits.
func NewID() (string, error) {
	var raw [5]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate Atomdown ID: %w", err)
	}

	value := uint64(raw[0])<<32 |
		uint64(raw[1])<<24 |
		uint64(raw[2])<<16 |
		uint64(raw[3])<<8 |
		uint64(raw[4])

	result := make([]byte, 8)
	for index := len(result) - 1; index >= 0; index-- {
		result[index] = crockfordBase32[value&31]
		value >>= 5
	}
	return string(result), nil
}
