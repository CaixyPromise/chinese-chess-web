package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPasswordUnicodeWhitespaceSaltAndBounds(t *testing.T) {
	s := &Service{hashSlots: make(chan struct{}, 2)}
	ctx := context.Background()
	password := strings.Repeat("棋", 14) + " "
	a, err := s.hashPassword(ctx, password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.hashPassword(ctx, password)
	if err != nil || a == b {
		t.Fatal("missing independent salts")
	}
	ok, err := s.checkPassword(ctx, password, a)
	if err != nil || !ok {
		t.Fatal("unicode password rejected")
	}
	ok, err = s.checkPassword(ctx, strings.TrimSpace(password), a)
	if err != nil || ok {
		t.Fatal("password was trimmed")
	}
	if validatePassword(strings.Repeat("棋", 14)) == nil || validatePassword(strings.Repeat("棋", 129)) == nil {
		t.Fatal("password bounds")
	}
	if _, err = s.hashPassword(ctx, strings.Repeat("x", 129)); err == nil {
		t.Fatal("oversize password")
	}
	for _, stored := range []string{"bad", "$argon2id$v=19$m=99999999,t=2,p=1$a$b"} {
		ok, err = s.checkPassword(ctx, password, stored)
		if err != nil || ok {
			t.Fatal("invalid encoded parameters accepted")
		}
	}
}
func TestLoginDelayDoesNotExtendWhileBlocked(t *testing.T) {
	l := newLimiter()
	for i := 0; i < 5; i++ {
		l.loginResult("a", false)
	}
	if l.loginAllowed("a") {
		t.Fatal("fifth failure did not delay")
	}
	first := l.entries["login:a"].next
	l.loginResult("a", false)
	if !l.entries["login:a"].next.Equal(first) {
		t.Fatal("blocked request extended lock")
	}
	l.mu.Lock()
	v := l.entries["login:a"]
	v.next = time.Now().Add(-time.Second)
	l.entries["login:a"] = v
	l.mu.Unlock()
	l.loginResult("a", true)
	if !l.loginAllowed("a") {
		t.Fatal("successful login did not reset")
	}
}
