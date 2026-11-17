package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email || len(email) > 254 || strings.ContainsAny(email, "\r\n") {
		return "", problem("INVALID_EMAIL", 400)
	}
	return email, nil
}
func validatePassword(p string) error {
	n := utf8.RuneCountInString(p)
	if !utf8.ValidString(p) || n < 15 || n > 128 || len(p) > 512 {
		return problem("INVALID_PASSWORD", 400)
	}
	return nil
}
func validateNickname(n string) error {
	if !utf8.ValidString(n) || utf8.RuneCountInString(n) < 1 || utf8.RuneCountInString(n) > 40 || strings.TrimSpace(n) == "" {
		return problem("INVALID_NICKNAME", 400)
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return problem("INVALID_NICKNAME", 400)
		}
	}
	return nil
}
func (s *Service) hashPassword(ctx context.Context, p string) (string, error) {
	if err := validatePassword(p); err != nil {
		return "", err
	}
	select {
	case s.hashSlots <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-s.hashSlots }()
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	hash := argon2.IDKey([]byte(p), salt, 2, 19*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}
func (s *Service) checkPassword(ctx context.Context, p, encoded string) (bool, error) {
	// An absent account performs the same bounded KDF, without admitting arbitrary
	// stored parameters or an unbounded incoming password into the hashing path.
	if len(p) > 512 {
		return false, nil
	}
	salt := make([]byte, 16)
	expected := make([]byte, 32)
	valid := false
	fields := strings.Split(encoded, "$")
	if len(fields) == 6 && fields[1] == "argon2id" && fields[2] == "v=19" && fields[3] == "m=19456,t=2,p=1" {
		a, e := base64.RawStdEncoding.DecodeString(fields[4])
		b, f := base64.RawStdEncoding.DecodeString(fields[5])
		if e == nil && f == nil && len(a) == 16 && len(b) == 32 {
			salt, expected, valid = a, b, true
		}
	}
	select {
	case s.hashSlots <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-s.hashSlots }()
	got := argon2.IDKey([]byte(p), salt, 2, 19*1024, 1, 32)
	return subtle.ConstantTimeCompare(got, expected) == 1 && valid, nil
}
