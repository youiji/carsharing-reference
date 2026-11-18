package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"carsharing/internal/models"
)

// StatsRepository — агрегирующие запросы для GET /api/stats.
type StatsRepository struct {
	db *pgxpool.Pool
}

func NewStatsRepository(db *pgxpool.Pool) *StatsRepository {
	return &StatsRepository{db: db}
}

// Get собирает статистику в одной транзакции REPEATABLE READ только для чтения:
// все запросы видят один и тот же снимок данных, поэтому цифры согласованы
// (например, аренда не попадёт в «забронировано» и одновременно в «завершено»).
func (r *StatsRepository) Get(ctx context.Context) (models.Stats, error) {
	var s models.Stats
	opts := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	err := pgx.BeginTxFunc(ctx, r.db, opts, func(tx pgx.Tx) error {
		var err error
		if s.UsersByRole, err = countBy(ctx, tx, `SELECT role, count(*) FROM users GROUP BY role`); err != nil {
			return fmt.Errorf("users by role: %w", err)
		}
		if s.CarsByStatus, err = countBy(ctx, tx, `SELECT status, count(*) FROM cars GROUP BY status`); err != nil {
			return fmt.Errorf("cars by status: %w", err)
		}
		if s.RentalsByStatus, err = countBy(ctx, tx, `SELECT status, count(*) FROM rentals GROUP BY status`); err != nil {
			return fmt.Errorf("rentals by status: %w", err)
		}

		// Среднее время от бронирования до старта по всем начатым поездкам
		// (active и completed), в минутах с одним знаком после запятой.
		err = tx.QueryRow(ctx, `
			SELECT round((avg(extract(epoch FROM started_at - booked_at)) / 60)::numeric, 1)::float8
			FROM rentals
			WHERE started_at IS NOT NULL`).Scan(&s.AvgWaitMinutes)
		if err != nil {
			return fmt.Errorf("avg wait: %w", err)
		}

		// Выручка: завершённые поездки и платные отмены бронирований (списанное ожидание).
		// Группировка по тарифу, сохранённому в аренде (снимок на момент бронирования).
		// Тарифы без выручки тоже выводятся — с нулём.
		rows, err := tx.Query(ctx, `
			SELECT t.name, count(r.id), COALESCE(sum(r.total_cost), 0)
			FROM tariffs t
			LEFT JOIN rentals r ON r.tariff_id = t.id
			     AND (r.status = 'completed' OR (r.status = 'cancelled' AND r.total_cost > 0))
			GROUP BY t.id, t.name
			ORDER BY 3 DESC, t.name`)
		if err != nil {
			return fmt.Errorf("revenue by tariff: %w", err)
		}
		s.RevenueByTariff, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.TariffRevenue, error) {
			var tr models.TariffRevenue
			err := row.Scan(&tr.Tariff, &tr.RentalsCount, &tr.Revenue)
			return tr, err
		})
		if err != nil {
			return fmt.Errorf("scan revenue: %w", err)
		}
		return nil
	})
	return s, err
}

// countBy выполняет запрос вида "SELECT ключ, count(*) ... GROUP BY ключ".
func countBy(ctx context.Context, tx pgx.Tx, sql string) (map[string]int, error) {
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	var (
		key string
		n   int
	)
	_, err = pgx.ForEachRow(rows, []any{&key, &n}, func() error {
		out[key] = n
		return nil
	})
	return out, err
}
