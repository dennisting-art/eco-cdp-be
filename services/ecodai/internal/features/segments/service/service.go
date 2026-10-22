package service

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"strings"

	pkgErrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
)

type Service struct {
	repo               Repository
	evaluator          Evaluator
	exportAvailability bool
}

func NewService(repo Repository, evaluator Evaluator, exportAvailability bool) *Service {
	return &Service{repo: repo, evaluator: evaluator, exportAvailability: exportAvailability}
}
func (s *Service) List(ctx context.Context, f model.ListFilter) ([]model.Segment, int64, error) {
	return s.repo.List(ctx, f)
}
func (s *Service) Get(ctx context.Context, id string) (*model.Segment, error) {
	return s.repo.Get(ctx, id)
}
func (s *Service) CreateSegment(ctx context.Context, in model.CreateSegmentInput, userID string) (*model.Segment, error) {
	if strings.TrimSpace(in.Name) == "" || len(in.Filters) == 0 {
		return nil, pkgErrors.InvalidRequest()
	}
	for _, tag := range in.Tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "predefine", "custom", "attribute":
			return nil, pkgErrors.InvalidRequest()
		}
	}
	if in.Operation == "" {
		in.Operation = "AND"
	}
	in.Operation = strings.ToUpper(in.Operation)
	if in.Operation != "AND" && in.Operation != "OR" {
		return nil, pkgErrors.InvalidRequest()
	}
	for _, f := range in.Filters {
		if strings.TrimSpace(f.Key) == "" || strings.TrimSpace(f.Operation) == "" || f.Values == nil {
			return nil, pkgErrors.InvalidRequest()
		}
	}
	return s.repo.CreateSegment(ctx, in, userID)
}
func (s *Service) Request(ctx context.Context, in model.RequestInput, userID string, idempotencyKey string) (*model.EstimateResult, *model.Request, error) {
	if err := validate(in); err != nil {
		return nil, nil, err
	}
	if in.Mode == "estimate" {
		if s.evaluator == nil {
			return nil, nil, pkgErrors.Internal()
		}
		if buildEvaluator, ok := s.evaluator.(interface {
			EstimateBuild(context.Context, model.RequestInput) (model.EstimateResult, error)
		}); ok {
			result, err := buildEvaluator.EstimateBuild(ctx, in)
			if err != nil {
				return nil, nil, err
			}
			return &result, nil, nil
		}
		count, err := s.evaluator.Estimate(ctx, in.Segments)
		if err != nil {
			return nil, nil, err
		}
		return &model.EstimateResult{Count: count}, nil, nil
	}
	if !s.exportAvailability {
		return nil, nil, pkgErrors.ServiceUnavailable()
	}
	r, err := s.repo.CreateConfirm(ctx, in, userID, idempotencyKey)
	return nil, r, err
}
func (s *Service) GetRequest(ctx context.Context, id string) (*model.Request, error) {
	return s.repo.GetRequest(ctx, id)
}
func validate(in model.RequestInput) error {
	if in.Base != "" && in.Base != "all_members" && !strings.HasPrefix(in.Base, "seg_") {
		return pkgErrors.InvalidRequest()
	}
	if in.Mode != "estimate" && in.Mode != "confirm" {
		return pkgErrors.InvalidRequest()
	}
	if len(in.Segments) == 0 {
		return pkgErrors.InvalidRequest()
	}
	if len(in.Segments) > 50 {
		return pkgErrors.InvalidRequest()
	}
	for i, segment := range in.Segments {
		if strings.TrimSpace(segment.ID) == "" {
			return pkgErrors.InvalidRequest()
		}
		op := strings.ToUpper(segment.Operation)
		if i == len(in.Segments)-1 {
			if op != "" {
				return pkgErrors.InvalidRequest()
			}
		}
		if i != len(in.Segments)-1 && op != "AND" && op != "OR" {
			return pkgErrors.InvalidRequest()
		}
		if len(segment.Filters) > 20 {
			return pkgErrors.InvalidRequest()
		}
		for _, f := range segment.Filters {
			if strings.TrimSpace(f.Key) == "" || f.Values == nil {
				return pkgErrors.InvalidRequest()
			}
		}
	}
	return nil
}
