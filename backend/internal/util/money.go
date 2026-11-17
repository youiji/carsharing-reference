package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Money — денежная сумма в копейках. float64 для денег не используется
// (docs/api.md, раздел 1.1): суммы хранятся и считаются в целых копейках.
//
// В JSON сумма пишется числом с двумя знаками после запятой (8.90),
// в PostgreSQL читается из NUMERIC и записывается в NUMERIC без потери точности.
type Money int64

// MoneyFromRubles создаёт сумму из рублей и копеек: MoneyFromRubles(8, 90) == 8.90.
func MoneyFromRubles(rub, kop int64) Money { return Money(rub*100 + kop) }

// String возвращает сумму в виде "331.50".
func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// MarshalJSON пишет сумму числом с двумя знаками после запятой.
func (m Money) MarshalJSON() ([]byte, error) { return []byte(m.String()), nil }

// ErrMoneyFormat — значение не является суммой с не более чем двумя знаками после запятой.
var ErrMoneyFormat = errors.New("сумма должна быть числом с не более чем двумя знаками после запятой")

// UnmarshalJSON принимает JSON-число (8.9, 8.90, 9) с не более чем двумя знаками
// после запятой. Строки, экспоненты и лишние знаки отклоняются.
func (m *Money) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] == '"' || bytes.ContainsAny(data, "eE") {
		return ErrMoneyFormat
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return ErrMoneyFormat
	}
	v, err := ParseMoney(num.String())
	if err != nil {
		return err
	}
	*m = v
	return nil
}

// ParseMoney разбирает строку вида "8.90", "8.9", "9", "-1.5".
func ParseMoney(s string) (Money, error) {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, hasDot := strings.Cut(s, ".")
	if intPart == "" || len(frac) > 2 || (hasDot && frac == "") || len(intPart) > 15 {
		return 0, ErrMoneyFormat
	}
	for len(frac) < 2 {
		frac += "0"
	}
	rub, err1 := strconv.ParseUint(intPart, 10, 63)
	kop, err2 := strconv.ParseUint(frac, 10, 8)
	if err1 != nil || err2 != nil {
		return 0, ErrMoneyFormat
	}
	v := Money(int64(rub)*100 + int64(kop))
	if neg {
		v = -v
	}
	return v, nil
}

// ScanNumeric читает значение NUMERIC из PostgreSQL (интерфейс pgtype.NumericScanner).
func (m *Money) ScanNumeric(n pgtype.Numeric) error {
	if !n.Valid {
		return errors.New("money: NULL нельзя прочитать в Money, используйте *Money")
	}
	if n.NaN || n.InfinityModifier != pgtype.Finite {
		return errors.New("money: недопустимое значение NUMERIC")
	}
	// значение = Int × 10^Exp; в копейках = Int × 10^(Exp+2)
	v := new(big.Int).Set(n.Int)
	shift := int64(n.Exp) + 2
	ten := big.NewInt(10)
	if shift >= 0 {
		v.Mul(v, new(big.Int).Exp(ten, big.NewInt(shift), nil))
	} else {
		div := new(big.Int).Exp(ten, big.NewInt(-shift), nil)
		var rem big.Int
		v.QuoRem(v, div, &rem)
		if rem.Sign() != 0 {
			return fmt.Errorf("money: в значении больше двух знаков после запятой")
		}
	}
	if !v.IsInt64() {
		return errors.New("money: значение слишком велико")
	}
	*m = Money(v.Int64())
	return nil
}

// NumericValue передаёт сумму в PostgreSQL как NUMERIC (интерфейс pgtype.NumericValuer).
func (m Money) NumericValue() (pgtype.Numeric, error) {
	return pgtype.Numeric{Int: big.NewInt(int64(m)), Exp: -2, Valid: true}, nil
}
