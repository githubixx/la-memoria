package usecase

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func GeneratePasswordHash(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}

	const (
		memory      = 65536
		iterations  = 3
		parallelism = 1
		saltLength  = 16
		hashLength  = 32
	)

	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, hashLength)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(_ context.Context, encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, fmt.Errorf("invalid Argon2id PHC")
	}
	parameters := map[string]uint32{}
	for _, pair := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return false, fmt.Errorf("invalid Argon2id parameters")
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return false, fmt.Errorf("invalid Argon2id parameters")
		}
		parameters[key] = uint32(parsed)
	}
	if parameters["m"] == 0 || parameters["t"] == 0 || parameters["p"] == 0 {
		return false, fmt.Errorf("invalid Argon2id parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid Argon2id salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid Argon2id hash")
	}
	got := argon2.IDKey([]byte(password), salt, parameters["t"], parameters["m"], uint8(parameters["p"]), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
