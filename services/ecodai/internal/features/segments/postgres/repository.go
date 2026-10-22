package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"regexp"
	"strings"
	"time"
)

type Repository struct {
	db        *database.PostgresDB
	versioned bool
}

var safeCookbookColumn = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func NewRepository(db *database.PostgresDB) *Repository {
	return &Repository{db: db}
}

func NewBuildRepository(db *database.PostgresDB) *Repository {
	return &Repository{db: db, versioned: true}
}

func NewEvaluator(db *database.PostgresDB) *Repository {
	return &Repository{db: db}
}

const segmentReadColumns = `s.id,s.name,s.tags,c.display_name,s.category_code,s.kind,s.definition_version,s.definition_status,
    s.description,COALESCE(bc.build_consented_count,p.member_count),
    CASE WHEN bc.build_id IS NOT NULL THEN 'computed' WHEN p.patch_id IS NOT NULL THEN 'patched' ELSE NULL END,
    CASE WHEN bc.build_id IS NOT NULL THEN 'build_consented' ELSE p.population_basis END,
    COALESCE(TO_CHAR(b.source_date,'YYYY-MM-DD'),TO_CHAR(p.as_of_date,'YYYY-MM-DD')),
    bc.build_id,s.filter_criteria,s.last_updated_at`
const segmentReadJoins = ` FROM segments_v0 s
    LEFT JOIN segment_attribute_category_v0 c ON c.code=s.category_code
    LEFT JOIN segment_count_patch_v0 p ON p.segment_id=s.id AND p.org_id='ALL' AND p.status='active'
    LEFT JOIN segment_current_build_v0 current_build ON current_build.singleton=TRUE
    LEFT JOIN segment_build_v0 b ON b.build_id=current_build.build_id AND b.status='READY'
    LEFT JOIN segment_build_count_v0 bc ON bc.build_id=b.build_id AND bc.segment_id=s.id
        AND bc.definition_version=s.definition_version AND bc.org_id='ALL'`

func scanSegment(row pgx.Row) (model.Segment, error) {
	var s model.Segment
	var last *time.Time
	err := row.Scan(&s.ID, &s.Name, &s.Tags, &s.Category, &s.CategoryCode, &s.Kind,
		&s.DefinitionVersion, &s.DefinitionStatus, &s.Description, &s.EstimatedMemberCount,
		&s.CountSource, &s.CountBasis, &s.CountAsOf, &s.CountBuildID, &s.FilterCriteria, &last)
	if err != nil {
		return model.Segment{}, err
	}
	s.LastUpdatedAt = last
	if err := populateResponseFilters(&s); err != nil {
		return model.Segment{}, err
	}
	return s, nil
}

func (r *Repository) List(ctx context.Context, f model.ListFilter) ([]model.Segment, int64, error) {
	conditions := []string{"s.is_visible=TRUE"}
	args := []interface{}{}
	n := 1
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(s.name) LIKE $%d", n))
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
		n++
	}
	if len(f.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("s.tags @> $%d", n))
		args = append(args, f.Tags)
		n++
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM segments_v0 s WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	q := fmt.Sprintf("SELECT "+segmentReadColumns+segmentReadJoins+" WHERE %s ORDER BY s.created_at DESC LIMIT $%d OFFSET $%d", where, n, n+1)
	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.Segment{}
	for rows.Next() {
		s, err := scanSegment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}
func (r *Repository) Get(ctx context.Context, id string) (*model.Segment, error) {
	s, err := scanSegment(r.db.Pool.QueryRow(ctx, "SELECT "+segmentReadColumns+segmentReadJoins+" WHERE s.id=$1 AND s.is_visible=TRUE", id))
	if err != nil {
		return nil, err
	}
	return &s, nil
}
func (r *Repository) CreateSegment(ctx context.Context, in model.CreateSegmentInput, userID string) (*model.Segment, error) {
	if in.Tags == nil {
		in.Tags = []string{}
	}
	in.Tags = append(in.Tags, "custom")
	for _, filter := range in.Filters {
		field, err := r.catalogField(ctx, filter.Key)
		if err != nil {
			return nil, apierrors.Wrap("10001", "invalid request", 400, err)
		}
		operator := strings.ToLower(strings.TrimSpace(filter.Operation))
		if !contains(field.AllowedOperators, operator) || (operator != "in" && operator != "=" && operator != "!=" && operator != ">" && operator != ">=" && operator != "<" && operator != "<=") {
			return nil, apierrors.InvalidRequest()
		}
		values, ok := filter.Values.([]interface{})
		if !ok || len(values) == 0 {
			return nil, apierrors.InvalidRequest()
		}
		for _, value := range values {
			if err := validateCatalogValue(field, value); err != nil {
				return nil, apierrors.Wrap("10001", "invalid request", 400, err)
			}
		}
	}
	criteria, err := json.Marshal(map[string]interface{}{"operation": in.Operation, "filters": in.Filters})
	if err != nil {
		return nil, err
	}
	var id string
	err = r.db.Pool.QueryRow(ctx, `INSERT INTO segments_v0 (id,name,tags,description,filter_criteria,created_by) VALUES ('seg_' || lpad(nextval('segment_v0_element_sequence')::text, 6, '0'),$1,$2,$3,$4,$5) RETURNING id`, in.Name, in.Tags, in.Description, criteria, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func populateResponseFilters(segment *model.Segment) error {
	var criteria struct {
		Filters []model.SegmentFilter `json:"filters"`
	}
	if len(segment.FilterCriteria) == 0 {
		return nil
	}
	if err := json.Unmarshal(segment.FilterCriteria, &criteria); err != nil {
		return err
	}
	segment.Filters = make([]model.ResponseFilter, 0, len(criteria.Filters))
	for _, filter := range criteria.Filters {
		values := filter.Values
		switch value := filter.Values.(type) {
		case []interface{}:
			values = value
		default:
			values = []interface{}{value}
		}
		segment.Filters = append(segment.Filters, model.ResponseFilter{Key: filter.Key, Values: values})
	}
	return nil
}
func (r *Repository) GetRequest(ctx context.Context, id string) (*model.Request, error) {
	var out model.Request
	var completed *time.Time
	err := r.db.Pool.QueryRow(ctx, `SELECT id,request_id,mode,status,segments_payload,total_count,consent_count,bucket,
		requested_at,completed_at,progress_stage,files_completed,files_total,bytes_uploaded,error_code,
		build_id,build_consent_count,consent_checked_at
		FROM segment_requests_v0 WHERE request_id=$1`, id).
		Scan(&out.ID, &out.RequestID, &out.Mode, &out.Status, &out.SegmentsPayload,
			&out.TotalCount, &out.ConsentCount, &out.Bucket, &out.RequestedAt, &completed,
			&out.Progress.Stage, &out.Progress.FilesCompleted, &out.Progress.FilesTotal,
			&out.Progress.BytesUploaded, &out.ErrorCode, &out.BuildID,
			&out.BuildConsentCount, &out.ConsentCheckedAt)
	if err != nil {
		return nil, err
	}
	out.CompletedAt = completed
	return &out, nil
}

var _ = pgx.ErrNoRows
