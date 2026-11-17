package services

import (
	"encoding/json"
	"slices"
	"testing"

	"carsharing/internal/dto"
	"carsharing/internal/models"
	"carsharing/internal/util"
)

func details(err error) map[string]string {
	if appErr, ok := util.AsAppError(err); ok {
		return appErr.Details
	}
	return nil
}

func TestValidateCar(t *testing.T) {
	var req dto.CarRequest
	json.Unmarshal([]byte(`{"tariff_id":1,"brand_model":" Kia Rio ","plate_number":"a 101 aa 777",
		"fuel_level":0,"latitude":55.75,"longitude":37.61}`), &req)
	car, err := validateCar(req)
	if err != nil {
		t.Fatalf("корректный запрос отклонён: %v", err)
	}
	if car.BrandModel != "Kia Rio" || car.PlateNumber != "А101АА777" || car.FuelLevel != 0 {
		t.Errorf("нормализация: %+v", car)
	}

	// Пустой запрос: ошибки по всем обязательным полям сразу.
	_, err = validateCar(dto.CarRequest{})
	want := []string{"tariff_id", "brand_model", "plate_number", "fuel_level", "latitude", "longitude"}
	for _, f := range want {
		if details(err)[f] == "" {
			t.Errorf("нет ошибки по полю %s: %v", f, details(err))
		}
	}

	json.Unmarshal([]byte(`{"tariff_id":-1,"brand_model":"X","plate_number":"А1",
		"fuel_level":101,"latitude":91,"longitude":-181}`), &req)
	_, err = validateCar(req)
	if d := details(err); len(d) != 4 || d["brand_model"] != "" {
		t.Errorf("ожидались ошибки диапазонов по 4 полям: %v", d)
	}
}

func TestValidateTariff(t *testing.T) {
	var req dto.TariffRequest
	json.Unmarshal([]byte(`{"name":" Эконом ","price_per_min":8.9,"waiting_per_min":0}`), &req)
	tr, err := validateTariff(req)
	if err != nil || tr.Name != "Эконом" || tr.PricePerMin != 890 || tr.WaitingPerMin != 0 {
		t.Fatalf("validateTariff: %+v, %v", tr, err)
	}

	_, err = validateTariff(dto.TariffRequest{})
	if d := details(err); d["name"] == "" || d["price_per_min"] == "" || d["waiting_per_min"] == "" {
		t.Errorf("пустой запрос: %v", d)
	}

	big, neg := util.Money(100_000_000), util.Money(-1)
	_, err = validateTariff(dto.TariffRequest{Name: "X", PricePerMin: &big, WaitingPerMin: &neg})
	if d := details(err); d["price_per_min"] == "" || d["waiting_per_min"] == "" {
		t.Errorf("цены вне диапазона: %v", d)
	}
}

// Таблица 2.2 api.md: из booked и in_use вручную не выйти,
// в booked и in_use вручную не попасть.
func TestManualCarTransitions(t *testing.T) {
	for _, from := range []string{models.CarBooked, models.CarInUse} {
		if len(manualCarTransitions[from]) != 0 {
			t.Errorf("из %s не должно быть ручных переходов", from)
		}
	}
	for from, tos := range manualCarTransitions {
		for _, to := range tos {
			if !slices.Contains(ManualCarStatuses, to) || to == from {
				t.Errorf("недопустимый переход %s → %s", from, to)
			}
		}
	}
	if slices.Contains(manualCarTransitions[models.CarDecommissioned], models.CarMaintenance) {
		t.Error("decommissioned → maintenance отсутствует в таблице 2.2")
	}
}
