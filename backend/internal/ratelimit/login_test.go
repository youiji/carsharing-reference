package ratelimit

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	l := NewLoginLimiter()
	clock := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }

	for i := 0; i < 5; i++ {
		if _, ok := l.Check("1.1.1.1", "Ivan@Example.com"); !ok {
			t.Fatalf("попытка %d не должна блокироваться", i+1)
		}
		l.Fail("1.1.1.1", "ivan@example.com")
	}
	wait, ok := l.Check("2.2.2.2", " IVAN@example.com ")
	if ok || wait != 15*time.Minute {
		t.Fatalf("после 5 неудач email блокируется на 15 минут независимо от IP и регистра: ok=%v wait=%v", ok, wait)
	}
	if _, ok := l.Check("1.1.1.1", "anna@example.com"); !ok {
		t.Error("другой email с того же IP ещё не исчерпал лимит IP (20)")
	}

	clock = clock.Add(15 * time.Minute)
	if _, ok := l.Check("1.1.1.1", "ivan@example.com"); !ok {
		t.Error("по истечении окна вход снова разрешён")
	}

	// Лимит по IP: 20 разных email с одного адреса.
	for i := 0; i < 20; i++ {
		l.Fail("3.3.3.3", string(rune('a'+i))+"@example.com")
	}
	if _, ok := l.Check("3.3.3.3", "new@example.com"); ok {
		t.Error("после 20 неудач с одного IP вход с него блокируется")
	}

	// Успешный вход сбрасывает счётчик email.
	for i := 0; i < 4; i++ {
		l.Fail("4.4.4.4", "maria@example.com")
	}
	l.Success("maria@example.com")
	for i := 0; i < 4; i++ {
		l.Fail("4.4.4.4", "maria@example.com")
	}
	if _, ok := l.Check("4.4.4.4", "maria@example.com"); !ok {
		t.Error("после успешного входа счётчик email начинается заново")
	}
}
