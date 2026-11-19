package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultParams = &params{
	memory:      1024 * 64, // 64 MB
	iterations:  2,         // 4 iterations
	parallelism: 2,         // 4 parallel threads
	saltLength:  16,        // 16 bytes
	keyLength:   32,        // 32 bytes
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, defaultParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		defaultParams.iterations,
		defaultParams.memory,
		defaultParams.parallelism,
		defaultParams.keyLength,
	)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		defaultParams.memory,
		defaultParams.iterations,
		defaultParams.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)

	return encoded, nil
}

func VerifyPassword(password, encodedHash string) (bool, error) {
	p, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	otherHash := argon2.IDKey(
		[]byte(password),
		salt,
		p.iterations,
		p.memory,
		p.parallelism,
		p.keyLength,
	)

	return subtle.ConstantTimeCompare(hash, otherHash) == 1, nil
}

func ValidatePasswordHash(encodedHash string) error {
	_, _, _, err := decodeHash(encodedHash)
	return err
}

func decodeHash(encodedHash string) (*params, []byte, []byte, error) {
	var p params
	var salt, hash []byte
	var version int

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return nil, nil, nil, fmt.Errorf("invalid hash format")
	}

	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, err
	}

	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, err
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, err
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, err
	}

	p.keyLength = uint32(len(hash))
	if parts[1] != "argon2id" || version != argon2.Version || p.parallelism == 0 ||
		p.memory < 8*uint32(p.parallelism) || p.memory > 1024*1024 ||
		p.iterations == 0 || p.iterations > 20 || len(salt) < 8 || len(hash) < 16 || len(hash) > 64 {
		return nil, nil, nil, fmt.Errorf("invalid or unsupported Argon2id parameters")
	}

	return &p, salt, hash, nil
}
