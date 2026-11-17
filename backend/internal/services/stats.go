package services

import (
	"context"

	"carsharing/internal/dto"
	"carsharing/internal/repository"
)

// StatsService — сводная статистика (docs/api.md, раздел 11).
type StatsService struct {
	stats *repository.StatsRepository
}

func NewStatsService(stats *repository.StatsRepository) *StatsService {
	return &StatsService{stats: stats}
}

// Get — GET /api/stats.
func (s *StatsService) Get(ctx context.Context) (dto.Stats, error) {
	st, err := s.stats.Get(ctx)
	if err != nil {
		return dto.Stats{}, err
	}
	return dto.NewStats(st), nil
}
