package services

import (
	"testing"
	"time"

	"carsharing/internal/util"
)

func TestCalcRentalCost(t *testing.T) {
	base := time.Date(2026, 10, 20, 9, 50, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return base.Add(d) }
	econ, econWait := util.Money(890), util.Money(200) // Эконом: 8.90 и 2.00

	tests := []struct {
		name           string
		started, ended time.Duration
		want           util.Money
	}{
		// Аренда 1 из db/seed.sql: ожидание 10 мин, поездка 35 мин → 331.50
		{"пример из seed.sql", 10 * time.Minute, 45 * time.Minute, 33150},
		{"неполная минута поездки округляется вверх", 10 * time.Minute, 45*time.Minute + time.Second, 33150 + 890},
		{"неполная минута ожидания округляется вверх", 10*time.Minute + time.Second, 45*time.Minute + time.Second, 33150 + 200},
		{"мгновенный старт и финиш — минимум 1 минута", 0, 0, 890},
		{"поездка 59 секунд — 1 минута", 0, 59 * time.Second, 890},
		{"ровно 2 минуты — без округления", 0, 2 * time.Minute, 2 * 890},
	}
	for _, tt := range tests {
		got := CalcRentalCost(base, at(tt.started), at(tt.ended), econ, econWait)
		if got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCeilMinutes(t *testing.T) {
	cases := map[time.Duration]int64{
		-time.Minute:                   0,
		0:                              0,
		time.Nanosecond:                1,
		time.Minute:                    1,
		time.Minute + time.Microsecond: 2,
		90 * time.Minute:               90,
	}
	for d, want := range cases {
		if got := ceilMinutes(d); got != want {
			t.Errorf("ceilMinutes(%v) = %d, want %d", d, got, want)
		}
	}
}

func TestCancellationFee(t *testing.T) {
	booked := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	waiting := util.Money(300) // 3.00 за минуту
	cases := []struct {
		name  string
		after time.Duration
		want  util.Money
	}{
		{"сразу после бронирования", 0, 0},
		{"неполная минута — одна минута", 10 * time.Second, 300},
		{"7 мин 30 с — 8 минут", 7*time.Minute + 30*time.Second, 2400},
		{"ровно срок бронирования", 20 * time.Minute, 6000},
		{"позже срока — не больше 20 минут", 45 * time.Minute, 6000},
	}
	for _, c := range cases {
		if got := CancellationFee(booked, booked.Add(c.after), waiting); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
