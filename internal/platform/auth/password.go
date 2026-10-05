package auth

import (
	"crypto/rand"
	"errors"
	"sync"
	"unicode/utf8"

	"github.com/alexedwards/argon2id"
)

// The policy refusals of CheckPassword.
var (
	ErrPasswordTooShort = errors.New("password too short")
	ErrPasswordTooLong  = errors.New("password too long")
)

const (
	passwordMinLength = 12
	passwordMaxLength = 1024
)

// HashParams are the pinned Argon2id parameters.
var HashParams = &argon2id.Params{Memory: 65536, Iterations: 3, Parallelism: 4, SaltLength: 16, KeyLength: 32}

// CheckPassword applies the policy: 12 to 1024 code points.
func CheckPassword(password string) error {
	switch n := utf8.RuneCountInString(password); {
	case n < passwordMinLength:
		return ErrPasswordTooShort
	case n > passwordMaxLength:
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword returns the PHC string of password under HashParams.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, HashParams)
}

// VerifyPassword reports whether password matches hash and whether the
// hash was made with other parameters than HashParams and should be
// written again. A hash that cannot be parsed matches nothing and needs
// a rehash.
func VerifyPassword(password, hash string) (match, needsRehash bool) {
	ok, params, err := argon2id.CheckHash(password, hash)
	if err != nil {
		return false, true
	}
	outdated := params.Memory != HashParams.Memory ||
		params.Iterations != HashParams.Iterations ||
		params.Parallelism != HashParams.Parallelism
	return ok, ok && outdated
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// DummyHash is a hash of a random password, made once per process, so
// that an unknown user costs one verification like a known one.
func DummyHash() string {
	dummyOnce.Do(func() {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic("auth: read random bytes: " + err.Error())
		}
		h, err := HashPassword(string(secret))
		if err != nil {
			panic("auth: hash dummy password: " + err.Error())
		}
		dummyHash = h
	})
	return dummyHash
}
