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
