package util

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ListQuery разбирает параметры списков: поиск, фильтры, сортировку
// (docs/api.md, раздел 1.3). Ошибки копятся и возвращаются одним
// ответом 400 validation_error.
type ListQuery struct {
	values url.Values
	v      Validator
}

func NewListQuery(values url.Values) *ListQuery {
	return &ListQuery{values: values}
}

// Search возвращает строку поиска q без пробелов по краям.
func (l *ListQuery) Search() string {
	return strings.TrimSpace(l.values.Get("q"))
}

// Enum возвращает значение параметра из списка допустимых или nil, если параметр не задан.
func (l *ListQuery) Enum(name string, allowed ...string) *string {
	s := strings.TrimSpace(l.values.Get(name))
	if s == "" {
		return nil
	}
	if !slices.Contains(allowed, s) {
		l.v.Add(name, "допустимые значения: "+strings.Join(allowed, ", "))
		return nil
	}
	return &s
}

// PositiveInt возвращает положительное целое или nil, если параметр не задан.
func (l *ListQuery) PositiveInt(name string) *int64 {
	s := strings.TrimSpace(l.values.Get(name))
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		l.v.Add(name, "должно быть положительным целым числом")
		return nil
	}
	return &n
}

// Sort возвращает поле сортировки из белого списка (по умолчанию первое
// из allowed) и признак обратного порядка.
func (l *ListQuery) Sort(allowed ...string) (field string, desc bool) {
	field = allowed[0]
	if s := strings.TrimSpace(l.values.Get("sort")); s != "" {
		if slices.Contains(allowed, s) {
			field = s
		} else {
			l.v.Add("sort", "допустимые значения: "+strings.Join(allowed, ", "))
		}
	}
	switch strings.TrimSpace(l.values.Get("order")) {
	case "", "asc":
	case "desc":
		desc = true
	default:
		l.v.Add("order", "допустимые значения: asc, desc")
	}
	return field, desc
}

// Err возвращает ошибку валидации или nil.
func (l *ListQuery) Err() error { return l.v.Err() }

// LikeContains готовит шаблон для ILIKE "содержит подстроку":
// символы %, _ и \ в пользовательском вводе экранируются.
func LikeContains(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// PathID читает положительный целый идентификатор из пути ({id}).
// Некорректный идентификатор означает несуществующий объект — 404.
func PathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrNotFound
	}
	return id, nil
}
