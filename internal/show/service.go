package show

import (
	"context"
	"fmt"
	"strings"
)

const (
	maxNameLen  = 255
	maxSeatLen  = 10
	maxSeatCount = 100_000
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name         string
	Seats        []string
	PricePaise   int64
	PerUserLimit int
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Show, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Show{}, fmt.Errorf("name is required")
	}
	if len(name) > maxNameLen {
		return Show{}, fmt.Errorf("name exceeds %d characters", maxNameLen)
	}
	if in.PricePaise < 0 {
		return Show{}, fmt.Errorf("price_paise must be non-negative")
	}
	perUserLimit := in.PerUserLimit
	if perUserLimit <= 0 {
		perUserLimit = 4
	}

	seatCodes, err := NormalizeSeatCodes(in.Seats)
	if err != nil {
		return Show{}, err
	}

	return s.repo.Create(ctx, name, in.PricePaise, perUserLimit, seatCodes)
}

// NormalizeSeatCodes validates and deduplicates seat codes (sorted for stable locking order).
func NormalizeSeatCodes(seats []string) ([]string, error) {
	if len(seats) == 0 {
		return nil, fmt.Errorf("at least one seat is required")
	}
	if len(seats) > maxSeatCount {
		return nil, fmt.Errorf("too many seats (max %d)", maxSeatCount)
	}
	seen := make(map[string]struct{}, len(seats))
	out := make([]string, 0, len(seats))
	for _, raw := range seats {
		code := strings.TrimSpace(raw)
		if code == "" {
			return nil, fmt.Errorf("seat codes must be non-empty")
		}
		if len(code) > maxSeatLen {
			return nil, fmt.Errorf("seat code %q exceeds %d characters", code, maxSeatLen)
		}
		if _, dup := seen[code]; dup {
			return nil, fmt.Errorf("duplicate seat code %q", code)
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out, nil
}
