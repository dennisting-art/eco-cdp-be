package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"strings"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
)

// BuildEvaluator evaluates one immutable READY build in PostgreSQL. Every
// intermediate set stays in SQL; only a bounded result or count reaches Go.
type BuildEvaluator struct {
	db   *database.PostgresDB
	repo *Repository
}

func NewBuildEvaluator(db *database.PostgresDB) *BuildEvaluator {
	return &BuildEvaluator{db: db, repo: &Repository{db: db}}
}

func (e *BuildEvaluator) current(ctx context.Context) (string, time.Time, error) {
	var buildID string
	var sourceDate time.Time
	err := e.db.Pool.QueryRow(ctx, `SELECT b.build_id,b.source_date FROM segment_current_build_v0 c
		JOIN segment_build_v0 b ON b.build_id=c.build_id AND b.status='READY'`).Scan(&buildID, &sourceDate)
	return buildID, sourceDate, err
}

func (e *BuildEvaluator) Estimate(ctx context.Context, selections []model.SegmentSelection) (int64, error) {
	result, err := e.EstimateBuild(ctx, model.RequestInput{Base: "all_members", Segments: selections})
	return result.Count, err
}

func (e *BuildEvaluator) EstimateBuild(ctx context.Context, in model.RequestInput) (model.EstimateResult, error) {
	buildID, sourceDate, err := e.current(ctx)
	if err != nil {
		return model.EstimateResult{}, err
	}
	if bitmapCount, supported, err := e.bitmapCount(ctx, buildID, in); err != nil {
		return model.EstimateResult{}, err
	} else if supported {
		return model.EstimateResult{Count: bitmapCount, BuildID: buildID,
			SourceDate: sourceDate.Format("2006-01-02")}, nil
	}
	query, args, err := e.CandidateSQL(ctx, buildID, sourceDate, in)
	if err != nil {
		return model.EstimateResult{}, err
	}
	var count int64
	err = e.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM (`+query+`) candidates
		JOIN member_facts_v0 m ON m.build_id=$1 AND m.member_id=candidates.member_id
		WHERE m.build_consent`, args...).Scan(&count)
	if err != nil {
		return model.EstimateResult{}, err
	}
	return model.EstimateResult{Count: count, BuildID: buildID, SourceDate: sourceDate.Format("2006-01-02")}, nil
}

// CandidateSQL returns the raw membership set. Consent is applied outside the
// set expression so an OR branch cannot bypass it.
func (e *BuildEvaluator) CandidateSQL(ctx context.Context, buildID string, sourceDate time.Time, in model.RequestInput) (string, []any, error) {
	if len(in.Segments) == 0 {
		return "", nil, fmt.Errorf("at least one segment is required")
	}
	args := []any{buildID, sourceDate}
	parts := []string{}
	base := in.Base
	if base == "" {
		base = "all_members"
	}
	if base == "all_members" {
		parts = append(parts, `s0 AS (SELECT m.member_id FROM member_facts_v0 m WHERE m.build_id=$1 AND $2::date IS NOT NULL)`)
	} else {
		baseSQL, err := e.segmentSQL(ctx, base, nil, &args)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, "s0 AS ("+baseSQL+")")
	}
	previous := "s0"
	for i, selection := range in.Segments {
		segmentSQL, err := e.segmentSQL(ctx, selection.ID, selection.Filters, &args)
		if err != nil {
			return "", nil, err
		}
		name := fmt.Sprintf("s%d", i+1)
		parts = append(parts, name+" AS ("+segmentSQL+")")
		op := "INTERSECT"
		if i > 0 && strings.EqualFold(in.Segments[i-1].Operation, "OR") {
			op = "UNION"
		}
		step := fmt.Sprintf("step%d", i+1)
		parts = append(parts, fmt.Sprintf("%s AS (SELECT member_id FROM %s %s SELECT member_id FROM %s)", step, previous, op, name))
		previous = step
	}
	return "WITH " + strings.Join(parts, ",") + " SELECT member_id FROM " + previous, args, nil
}

func (e *BuildEvaluator) segmentSQL(ctx context.Context, id string, selected []model.SegmentFilter, args *[]any) (string, error) {
	var kind, status string
	var category *string
	var fixedRule []byte
	var rawCriteria []byte
	err := e.db.Pool.QueryRow(ctx, `SELECT kind,definition_status,category_code,fixed_rule,filter_criteria
		FROM segments_v0 WHERE id=$1 AND is_visible`, id).
		Scan(&kind, &status, &category, &fixedRule, &rawCriteria)
	if err != nil {
		return "", err
	}
	if status == "retired" {
		return "", fmt.Errorf("segment %s is retired", id)
	}
	joins := ""
	conditions := []string{"m.build_id=$1", "$2::date IS NOT NULL"}
	if kind == "predefined" && len(fixedRule) == 0 {
		joins = ` JOIN member_predefined_v0 mp ON mp.build_id=m.build_id AND mp.member_id=m.member_id`
		conditions = append(conditions, "mp.segment_id="+addArg(args, id))
	} else if kind == "option" && category != nil && (*category == "GENDER" || *category == "AGE_BAND" || *category == "RACE") {
		column := map[string]string{"GENDER": "gender_option_id", "AGE_BAND": "age_option_id", "RACE": "race_option_id"}[*category]
		conditions = append(conditions, "m."+column+"="+addArg(args, id))
	} else if len(fixedRule) != 0 {
		next := len(*args) + 1
		condition, err := attributeRuleCondition(fixedRule, args, &next)
		if err != nil {
			return "", err
		}
		conditions = append(conditions, condition)
	} else if kind != "custom" {
		return "", fmt.Errorf("segment %s has no build-scoped rule", id)
	}
	var definition struct {
		Operation string                `json:"operation"`
		Filters   []model.SegmentFilter `json:"filters"`
	}
	if err := json.Unmarshal(rawCriteria, &definition); err != nil {
		return "", err
	}
	if kind == "custom" {
		if len(definition.Filters) == 0 {
			return "", fmt.Errorf("custom segment %s has no filters", id)
		}
		filterSQL, err := e.filtersSQL(ctx, definition.Filters, definition.Operation, args)
		if err != nil {
			return "", err
		}
		conditions = append(conditions, filterSQL)
	}
	if len(selected) > 0 {
		allowed := map[string]model.SegmentFilter{}
		for _, filter := range definition.Filters {
			allowed[filter.Key] = filter
		}
		for _, filter := range selected {
			approved, ok := allowed[filter.Key]
			if !ok {
				return "", fmt.Errorf("filter %s is not enabled for %s", filter.Key, id)
			}
			value, err := validateAndNormalizeSegmentFilter(segmentV0FilterDefinition{
				Key: approved.Key, Operation: approved.Operation, Values: approved.Values,
			}, filter.Values)
			if err != nil {
				return "", err
			}
			filter.Operation = approved.Operation
			filter.Values = value
			predicate, err := e.filterSQL(ctx, filter, args)
			if err != nil {
				return "", err
			}
			conditions = append(conditions, predicate)
		}
	}
	return "SELECT m.member_id FROM member_facts_v0 m" + joins + " WHERE " + strings.Join(conditions, " AND "), nil
}

func (e *BuildEvaluator) filtersSQL(ctx context.Context, filters []model.SegmentFilter, operation string, args *[]any) (string, error) {
	op := strings.ToUpper(operation)
	if op == "" {
		op = "AND"
	}
	if op != "AND" && op != "OR" {
		return "", fmt.Errorf("invalid filter combination %q", operation)
	}
	parts := make([]string, 0, len(filters))
	for _, filter := range filters {
		part, err := e.filterSQL(ctx, filter, args)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return "(" + strings.Join(parts, " "+op+" ") + ")", nil
}

func (e *BuildEvaluator) filterSQL(ctx context.Context, filter model.SegmentFilter, args *[]any) (string, error) {
	field, err := e.repo.catalogField(ctx, filter.Key)
	if err != nil {
		return "", err
	}
	op := strings.ToLower(strings.TrimSpace(filter.Operation))
	if !contains(field.AllowedOperators, op) ||
		(op != "in" && op != "=" && op != "!=" && op != ">" && op != ">=" && op != "<" && op != "<=") {
		return "", fmt.Errorf("operator %s is not allowed for %s", op, filter.Key)
	}
	values, ok := filter.Values.([]any)
	if !ok {
		values = []any{filter.Values}
	}
	if len(values) == 0 || len(values) > 50 {
		return "", fmt.Errorf("invalid value count for %s", filter.Key)
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if err := validateCatalogValue(field, value); err != nil {
			return "", err
		}
		operator := op
		if operator == "in" {
			operator = "="
		}
		if operator == "!=" {
			operator = "<>"
		}
		parts = append(parts, "m."+field.Column+" "+operator+" "+addArg(args, value))
	}
	combine := " OR "
	if op == "!=" {
		combine = " AND "
	}
	return "(" + strings.Join(parts, combine) + ")", nil
}

func addArg(args *[]any, value any) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}
