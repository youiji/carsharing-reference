package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"carsharing/internal/models"
)

func TestNewStatsZerosAndTotal(t *testing.T) {
	s := NewStats(models.Stats{
		UsersByRole:  map[string]int{"client": 6, "admin": 1},
		CarsByStatus: map[string]int{"available": 3},
	})
	if s.UsersTotal != 7 || s.UsersByRole["manager"] != 0 || len(s.UsersByRole) != 3 {
		t.Errorf("users: %+v", s)
	}
	if len(s.CarsByStatus) != 5 || len(s.RentalsByStatus) != 4 {
		t.Errorf("все статусы должны присутствовать: %v %v", s.CarsByStatus, s.RentalsByStatus)
	}
	out, _ := json.Marshal(s)
	if !strings.Contains(string(out), `"avg_wait_minutes":null`) || !strings.Contains(string(out), `"revenue_by_tariff":[]`) {
		t.Errorf("пустые значения: %s", out)
	}
}
