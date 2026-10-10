package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"runtime"
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

// maxHashSlots caps the hashes in flight: each Argon2id hash takes 64 MiB,
// so the memory held by hashes stays at or below maxHashSlots times that.
const maxHashSlots = 4

// hashSlots bounds the hashes in flight by the processors and by
// maxHashSlots, never by the number of requests.
var hashSlots = make(chan struct{}, min(runtime.GOMAXPROCS(0), maxHashSlots))

// withHashSlot runs fn while holding one of hashSlots. A wait that ctx
// ends returns its error and runs nothing.
func withHashSlot(ctx context.Context, fn func()) error {
	select {
	case hashSlots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-hashSlots }()
	fn()
	return nil
}

// HashPassword returns the PHC string of password under HashParams. It
// fails with the error of ctx when ctx ends while it waits for a slot.
func HashPassword(ctx context.Context, password string) (hash string, err error) {
	var hashErr error
	if err := withHashSlot(ctx, func() { hash, hashErr = argon2id.CreateHash(password, HashParams) }); err != nil {
		return "", err
	}
	return hash, hashErr
}

// VerifyPassword reports whether password matches hash and whether the
// hash was made with other parameters than HashParams and should be
// written again. A hash that cannot be parsed matches nothing and needs
// a rehash. It fails with the error of ctx when ctx ends while it waits
// for a slot.
func VerifyPassword(ctx context.Context, password, hash string) (match, needsRehash bool, err error) {
	var ok bool
	var params *argon2id.Params
	var checkErr error
	if err := withHashSlot(ctx, func() { ok, params, checkErr = argon2id.CheckHash(password, hash) }); err != nil {
		return false, false, err
	}
	if checkErr != nil {
		return false, true, nil
	}
	outdated := params.Memory != HashParams.Memory ||
		params.Iterations != HashParams.Iterations ||
		params.Parallelism != HashParams.Parallelism
	return ok, ok && outdated, nil
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
		h, err := HashPassword(context.Background(), string(secret))
		if err != nil {
			panic("auth: hash dummy password: " + err.Error())
		}
		dummyHash = h
	})
	return dummyHash
}
