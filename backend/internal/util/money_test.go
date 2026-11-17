package util

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestMoneyJSON(t *testing.T) {
	out, _ := json.Marshal(map[string]Money{"a": 890, "b": 33150, "c": 0, "d": -150})
	if string(out) != `{"a":8.90,"b":331.50,"c":0.00,"d":-1.50}` {
		t.Errorf("Marshal = %s", out)
	}

	ok := map[string]Money{"8.9": 890, "8.90": 890, "9": 900, "0": 0, "999999.99": 99999999, "-1.5": -150}
	for in, want := range ok {
		var m Money
		if err := json.Unmarshal([]byte(in), &m); err != nil || m != want {
			t.Errorf("Unmarshal(%s) = %v, %v; want %v", in, m, err, want)
		}
	}
	for _, in := range []string{`"8.90"`, "8.901", "1e2", "8.", "null_", "true"} {
		var m Money
		if err := json.Unmarshal([]byte(in), &m); err == nil {
			t.Errorf("Unmarshal(%s) должен вернуть ошибку, получено %v", in, m)
		}
	}
}

func TestMoneyNumeric(t *testing.T) {
	cases := []struct {
		n    pgtype.Numeric
		want Money
	}{
		{pgtype.Numeric{Int: big.NewInt(890), Exp: -2, Valid: true}, 890},   // 8.90
		{pgtype.Numeric{Int: big.NewInt(89), Exp: -1, Valid: true}, 890},    // 8.9
		{pgtype.Numeric{Int: big.NewInt(12), Exp: 3, Valid: true}, 1200000}, // 12000
		{pgtype.Numeric{Int: big.NewInt(33150), Exp: -2, Valid: true}, 33150},
	}
	for _, c := range cases {
		var m Money
		if err := m.ScanNumeric(c.n); err != nil || m != c.want {
			t.Errorf("ScanNumeric(%v e%d) = %v, %v; want %v", c.n.Int, c.n.Exp, m, err, c.want)
		}
	}
	var m Money
	if err := m.ScanNumeric(pgtype.Numeric{Int: big.NewInt(1234), Exp: -3, Valid: true}); err == nil {
		t.Error("три знака после запятой должны давать ошибку")
	}
	v, _ := Money(33150).NumericValue()
	if v.Int.Int64() != 33150 || v.Exp != -2 {
		t.Errorf("NumericValue = %v e%d", v.Int, v.Exp)
	}
}
