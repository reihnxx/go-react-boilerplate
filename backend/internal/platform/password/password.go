// Package password hashes and verifies passwords with argon2id and enforces
// the password policy. Legacy bcrypt hashes (e.g. imported from Laravel) are
// accepted for verification and flagged for re-hashing.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	MinLength = 8
	MaxLength = 128
)

type params struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLen     int
	keyLen      uint32
}

// OWASP-recommended argon2id baseline (19 MiB, t=2, p=1).
var current = params{memory: 19 * 1024, iterations: 2, parallelism: 1, saltLen: 16, keyLen: 32}

var errMalformed = errors.New("password: malformed hash")

var b64 = base64.RawStdEncoding

// Hash returns an encoded argon2id hash in PHC string format.
func Hash(plain string) (string, error) {
	salt := make([]byte, current.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, current.iterations, current.memory, current.parallelism, current.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, current.memory, current.iterations, current.parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify checks plain against encoded. needsRehash is true when the password
// matched but the stored hash uses an outdated algorithm or parameters.
func Verify(plain, encoded string) (match bool, needsRehash bool, err error) {
	if strings.HasPrefix(encoded, "$2") {
		err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(plain))
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, false, nil
		}
		if err != nil {
			return false, false, err
		}
		return true, true, nil
	}

	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	other := argon2.IDKey([]byte(plain), salt, p.iterations, p.memory, p.parallelism, uint32(len(key)))
	if subtle.ConstantTimeCompare(key, other) != 1 {
		return false, false, nil
	}
	stale := p.memory != current.memory || p.iterations != current.iterations || p.parallelism != current.parallelism
	return true, stale, nil
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// VerifyDummy spends the same time as a real verification. Call it when the
// user does not exist so response timing does not reveal valid logins.
func VerifyDummy(plain string) {
	dummyOnce.Do(func() { dummyHash, _ = Hash("timing-equalizer-password-1") })
	_, _, _ = Verify(plain, dummyHash)
}

// Validate returns a user-facing message when plain violates the policy, or
// "" when it is acceptable.
func Validate(plain string) string {
	n := utf8.RuneCountInString(plain)
	if n < MinLength {
		return fmt.Sprintf("Kata sandi minimal %d karakter.", MinLength)
	}
	if n > MaxLength {
		return fmt.Sprintf("Kata sandi maksimal %d karakter.", MaxLength)
	}
	var letter, digit bool
	for _, r := range plain {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !letter || !digit {
		return "Kata sandi harus mengandung huruf dan angka."
	}
	return ""
}

func decode(encoded string) (params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	if len(parts) != 6 || parts[1] != "argon2id" {
		return params{}, nil, nil, errMalformed
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return params{}, nil, nil, errMalformed
	}
	var p params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return params{}, nil, nil, errMalformed
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return params{}, nil, nil, errMalformed
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return params{}, nil, nil, errMalformed
	}
	return p, salt, key, nil
}
