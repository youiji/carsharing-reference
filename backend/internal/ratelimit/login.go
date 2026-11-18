// Package ratelimit ограничивает попытки входа: защита от подбора пароля.
package ratelimit

import (
	"strings"
	"sync"
	"time"
)

// LoginLimiter считает неудачные попытки входа отдельно по email и по IP-адресу
// за скользящее окно. Пока лимит исчерпан, вход блокируется даже с верным паролем.
// Счётчики хранятся в памяти процесса (для одного экземпляра сервера этого достаточно).
type LoginLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	perEmail int
	perIP    int
	fails    map[string][]time.Time
	calls    int
	now      func() time.Time
}

// NewLoginLimiter: не больше 5 неудачных попыток на один email и 20 на один
// IP-адрес за 15 минут.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		window:   15 * time.Minute,
		perEmail: 5,
		perIP:    20,
		fails:    map[string][]time.Time{},
		now:      time.Now,
	}
}

func emailKey(email string) string { return "email:" + strings.ToLower(strings.TrimSpace(email)) }
func ipKey(ip string) string       { return "ip:" + ip }

// Check возвращает false и время ожидания, если для email или IP лимит исчерпан.
func (l *LoginLimiter) Check(ip, email string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	wait := max(l.blocked(ipKey(ip), l.perIP, now), l.blocked(emailKey(email), l.perEmail, now))
	return wait, wait == 0
}

// Fail записывает неудачную попытку.
func (l *LoginLimiter) Fail(ip, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range []string{ipKey(ip), emailKey(email)} {
		l.fails[k] = append(l.prune(k, now), now)
	}
	l.calls++
	if l.calls%1000 == 0 { // изредка удаляем устаревшие ключи, чтобы карта не росла
		for k := range l.fails {
			if len(l.prune(k, now)) == 0 {
				delete(l.fails, k)
			}
		}
	}
}

// Success сбрасывает счётчик email после успешного входа.
func (l *LoginLimiter) Success(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, emailKey(email))
}

// blocked — сколько ещё ждать, если в окне уже limit неудачных попыток (0 — можно).
func (l *LoginLimiter) blocked(key string, limit int, now time.Time) time.Duration {
	list := l.prune(key, now)
	if len(list) < limit {
		return 0
	}
	return list[len(list)-limit].Add(l.window).Sub(now)
}

// prune оставляет попытки внутри окна.
func (l *LoginLimiter) prune(key string, now time.Time) []time.Time {
	list := l.fails[key]
	i := 0
	for i < len(list) && now.Sub(list[i]) >= l.window {
		i++
	}
	list = list[i:]
	if len(list) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = list
	}
	return list
}
