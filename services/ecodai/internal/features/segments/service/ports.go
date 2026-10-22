package service

import (
	"context"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
)

type Repository interface {
	List(context.Context, model.ListFilter) ([]model.Segment, int64, error)
	Get(context.Context, string) (*model.Segment, error)
	CreateSegment(context.Context, model.CreateSegmentInput, string) (*model.Segment, error)
	CreateConfirm(context.Context, model.RequestInput, string, string) (*model.Request, error)
	GetRequest(context.Context, string) (*model.Request, error)
}

type Evaluator interface {
	Estimate(context.Context, []model.SegmentSelection) (int64, error)
}
