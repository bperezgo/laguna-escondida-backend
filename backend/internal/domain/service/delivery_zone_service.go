package service

import (
	"context"
	"fmt"
	"sort"

	deliveryZone "laguna-escondida/backend/internal/domain/aggregate/delivery_zone"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type DeliveryZoneService struct {
	repo ports.DeliveryZoneRepository
}

func NewDeliveryZoneService(repo ports.DeliveryZoneRepository) *DeliveryZoneService {
	return &DeliveryZoneService{repo: repo}
}

func (s *DeliveryZoneService) List(ctx context.Context) ([]*dto.DeliveryZone, error) {
	zones, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryZoneOperation, err)
	}
	return zones, nil
}

func (s *DeliveryZoneService) Create(ctx context.Context, req *dto.CreateDeliveryZoneRequest) (*dto.DeliveryZone, error) {
	zone, err := deliveryZone.NewAggregateFromCreateRequest(req)
	if err != nil {
		return nil, err
	}
	zoneDTO := zone.ToDTO()
	if err := s.repo.Create(ctx, zoneDTO); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryZoneOperation, err)
	}
	return zoneDTO, nil
}

// Update never touches orders: they keep the zone id they were accepted in.
func (s *DeliveryZoneService) Update(ctx context.Context, id string, req *dto.UpdateDeliveryZoneRequest) (*dto.DeliveryZone, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	zone, err := deliveryZone.NewAggregateFromDTO(existing)
	if err != nil {
		return nil, err
	}
	if err := zone.Update(req); err != nil {
		return nil, err
	}
	zoneDTO := zone.ToDTO()
	if err := s.repo.Update(ctx, zoneDTO); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryZoneOperation, err)
	}
	return zoneDTO, nil
}

func (s *DeliveryZoneService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrDeliveryZoneOperation, err)
	}
	return nil
}

func (s *DeliveryZoneService) Check(ctx context.Context, lat, lng float64) (*dto.DeliveryZoneCheckResult, error) {
	if err := deliveryZone.ValidatePoint(lat, lng); err != nil {
		return nil, err
	}
	zones, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrDeliveryZoneOperation, err)
	}
	zone, ok := findContainingZone(zones, lat, lng)
	if !ok {
		return &dto.DeliveryZoneCheckResult{Deliverable: false}, nil
	}
	return &dto.DeliveryZoneCheckResult{Deliverable: true, ZoneID: &zone.ID, ZoneName: &zone.Name}, nil
}

// findContainingZone returns the oldest active zone containing the point. Oldest wins on
// overlap so the choice is deterministic; it matters once zones carry fees.
func findContainingZone(zones []*dto.DeliveryZone, lat, lng float64) (*dto.DeliveryZone, bool) {
	ordered := make([]*dto.DeliveryZone, len(zones))
	copy(ordered, zones)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].CreatedAt.Before(ordered[j].CreatedAt) })

	for _, z := range ordered {
		zone, err := deliveryZone.NewAggregateFromDTO(z)
		if err != nil {
			// A stored boundary that no longer validates delivers nowhere (fail closed).
			continue
		}
		if zone.Contains(lat, lng) {
			return z, true
		}
	}
	return nil, false
}
