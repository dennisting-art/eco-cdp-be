package model

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type Segment struct {
	ID                   string           `json:"id"`
	Name                 string           `json:"name"`
	Tags                 []string         `json:"tags"`
	Category             *string          `json:"category,omitempty"`
	CategoryCode         *string          `json:"category_code,omitempty"`
	Kind                 string           `json:"kind"`
	DefinitionVersion    int              `json:"definition_version"`
	DefinitionStatus     string           `json:"definition_status"`
	Description          *string          `json:"description"`
	EstimatedMemberCount *int64           `json:"estimated_count"`
	CountSource          *string          `json:"count_source"`
	CountBasis           *string          `json:"count_basis,omitempty"`
	CountAsOf            *string          `json:"count_as_of,omitempty"`
	CountBuildID         *string          `json:"count_build_id,omitempty"`
	FilterCriteria       json.RawMessage  `json:"-"`
	Filters              []ResponseFilter `json:"filters"`
	LastUpdatedAt        *time.Time       `json:"last_updated_at"`
}

type ListFilter struct {
	Search         string
	Tags           []string
	Page, PageSize int
}
type SegmentFilter struct {
	Key       string      `json:"key"`
	Operation string      `json:"operation"`
	Values    interface{} `json:"values"`
}

type ResponseFilter struct {
	Key    string      `json:"key"`
	Values interface{} `json:"values"`
}
type CreateSegmentInput struct {
	Name        string          `json:"name"`
	Tags        []string        `json:"tags"`
	Description *string         `json:"description"`
	Operation   string          `json:"operation"`
	Filters     []SegmentFilter `json:"filters"`
}

// SegmentSelection.Operation combines this item with the next item. The final item omits it.
type SegmentSelection struct {
	ID        string          `json:"id"`
	Filters   []SegmentFilter `json:"filters"`
	Operation string          `json:"operation,omitempty"`
}
type RequestInput struct {
	Mode     string             `json:"mode"`
	Base     string             `json:"base,omitempty"`
	Segments []SegmentSelection `json:"segments"`
}
type Request struct {
	ID                       uuid.UUID
	RequestID, Mode, Status  string
	SegmentsPayload          json.RawMessage
	TotalCount, ConsentCount *int64
	Bucket                   *string
	BuildID                  *string
	BuildConsentCount        *int64
	ConsentCheckedAt         *time.Time
	RequestedAt              time.Time
	CompletedAt              *time.Time
	Progress                 Progress `json:"progress"`
	ErrorCode                *string  `json:"error_code,omitempty"`
}
type Progress struct {
	Stage          string `json:"stage"`
	FilesCompleted int    `json:"files_completed"`
	FilesTotal     int    `json:"files_total"`
	BytesUploaded  int64  `json:"bytes_uploaded"`
}
type EstimateResult struct {
	Count      int64  `json:"count"`
	BuildID    string `json:"build_id,omitempty"`
	SourceDate string `json:"source_date,omitempty"`
}
