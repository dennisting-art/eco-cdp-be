package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"strings"
	"time"
)

type v0CatalogField struct {
	Column           string
	AllowedOperators []string
	DataType         string
	EnumValues       []string
}

type segmentV0FilterDefinition struct {
	Key       string      `json:"key"`
	Operation string      `json:"operation"`
	Values    interface{} `json:"values"`
}

type segmentV0Definition struct {
	Filters   map[string]segmentV0FilterDefinition
	BaseID    string
	FixedRule json.RawMessage
}

func (r *Repository) Estimate(ctx context.Context, selections []model.SegmentSelection) (int64, error) {
	sets := make([]map[string]struct{}, 0, len(selections))
	for _, selection := range selections {
		ids, err := r.matchConsentMembers(ctx, selection)
		if err != nil {
			return 0, err
		}
		set := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			set[id] = struct{}{}
		}
		sets = append(sets, set)
	}
	if len(sets) == 0 {
		return 0, nil
	}
	result := sets[0]
	for i := 0; i < len(sets)-1; i++ {
		if strings.EqualFold(selections[i].Operation, "AND") {
			for id := range result {
				if _, ok := sets[i+1][id]; !ok {
					delete(result, id)
				}
			}
			continue
		}
		for id := range sets[i+1] {
			result[id] = struct{}{}
		}
	}
	count := int64(len(result))
	return count, nil
}

func (r *Repository) matchConsentMembers(ctx context.Context, selection model.SegmentSelection) ([]string, error) {
	return r.matchMembers(ctx, selection, true)
}

func (r *Repository) matchMembers(ctx context.Context, selection model.SegmentSelection, consentOnly bool) ([]string, error) {
	return r.MatchMembersAt(ctx, selection, consentOnly, nil)
}

func (r *Repository) MatchMembersAt(ctx context.Context, selection model.SegmentSelection, consentOnly bool, pinned *time.Time) ([]string, error) {
	definition, err := r.segmentDefinition(ctx, selection.ID)
	if err != nil {
		return nil, err
	}
	snapshotDate := pinned
	if snapshotDate == nil {
		if err := r.db.Pool.QueryRow(ctx, `SELECT MAX(computed_date) FROM segment_members_v0 WHERE segment_id=$1`, definition.BaseID).Scan(&snapshotDate); err != nil {
			return nil, fmt.Errorf("load latest segment snapshot: %w", err)
		}
	}
	if snapshotDate == nil {
		return []string{}, nil
	}

	conditions := []string{"sm.segment_id = $1", "sm.computed_date = $2"}
	if consentOnly {
		conditions = append(conditions, "m.consent_flag = TRUE")
	}
	args := []interface{}{definition.BaseID, *snapshotDate}
	n := 3
	if len(definition.FixedRule) != 0 {
		fixedCondition, err := attributeRuleCondition(definition.FixedRule, &args, &n)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, fixedCondition)
	}
	for _, submitted := range selection.Filters {
		filterDefinition, ok := definition.Filters[submitted.Key]
		if !ok {
			return nil, fmt.Errorf("filter %q is not enabled for segment %q", submitted.Key, selection.ID)
		}
		values, err := validateAndNormalizeSegmentFilter(filterDefinition, submitted.Values)
		if err != nil {
			return nil, err
		}
		filter := model.SegmentFilter{Key: submitted.Key, Operation: filterDefinition.Operation, Values: values}
		field, err := r.catalogField(ctx, filter.Key)
		if err != nil {
			return nil, err
		}
		column := "m." + field.Column
		op := strings.ToLower(strings.TrimSpace(filter.Operation))
		if !contains(field.AllowedOperators, op) {
			return nil, fmt.Errorf("operator %q is not allowed for filter %q", filter.Operation, filter.Key)
		}
		if op == "in" {
			values, ok := filter.Values.([]interface{})
			if !ok || len(values) == 0 {
				return nil, fmt.Errorf("filter %q requires a non-empty values array", filter.Key)
			}
			placeholders := make([]string, 0, len(values))
			for _, value := range values {
				if err := validateCatalogValue(field, value); err != nil {
					return nil, err
				}
				placeholders = append(placeholders, fmt.Sprintf("$%d", n))
				args = append(args, value)
				n++
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ",")))
			continue
		}
		if op != "=" && op != "!=" && op != ">" && op != ">=" && op != "<" && op != "<=" {
			return nil, fmt.Errorf("unsupported v0 filter operator %q", filter.Operation)
		}
		if filter.Values == nil {
			return nil, fmt.Errorf("filter %q has no values", filter.Key)
		}
		if err := validateCatalogValue(field, filter.Values); err != nil {
			return nil, err
		}
		conditions = append(conditions, fmt.Sprintf("%s %s $%d", column, op, n))
		args = append(args, filter.Values)
		n++
	}
	query := "SELECT sm.member_id FROM segment_members_v0 sm JOIN member_cookbook_v0 m ON m.member_id=sm.member_id WHERE " + strings.Join(conditions, " AND ")
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) segmentDefinition(ctx context.Context, segmentID string) (segmentV0Definition, error) {
	var raw json.RawMessage
	var baseID *string
	var fixedRule json.RawMessage
	if err := r.db.Pool.QueryRow(ctx, `SELECT filter_criteria,base_segment_id,fixed_rule FROM segments_v0 WHERE id=$1 AND is_visible=TRUE`, segmentID).Scan(&raw, &baseID, &fixedRule); err != nil {
		if err == pgx.ErrNoRows {
			return segmentV0Definition{}, fmt.Errorf("segment %q not found", segmentID)
		}
		return segmentV0Definition{}, err
	}
	var criteria struct {
		Filters []segmentV0FilterDefinition `json:"filters"`
	}
	if err := json.Unmarshal(raw, &criteria); err != nil {
		return segmentV0Definition{}, fmt.Errorf("decode filters for segment %q: %w", segmentID, err)
	}
	definitions := make(map[string]segmentV0FilterDefinition, len(criteria.Filters))
	for _, definition := range criteria.Filters {
		if definition.Key == "" || definition.Operation == "" || definition.Values == nil {
			return segmentV0Definition{}, fmt.Errorf("invalid filter definition for segment %q", segmentID)
		}
		if _, exists := definitions[definition.Key]; exists {
			return segmentV0Definition{}, fmt.Errorf("duplicate filter definition %q for segment %q", definition.Key, segmentID)
		}
		definitions[definition.Key] = definition
	}
	result := segmentV0Definition{Filters: definitions, BaseID: segmentID, FixedRule: fixedRule}
	if baseID != nil {
		result.BaseID = *baseID
	}
	return result, nil
}

func validateAndNormalizeSegmentFilter(definition segmentV0FilterDefinition, selected interface{}) (interface{}, error) {
	allowed, ok := definition.Values.([]interface{})
	if !ok || len(allowed) == 0 {
		return nil, fmt.Errorf("invalid allowed values for filter %q", definition.Key)
	}
	op := strings.ToLower(definition.Operation)
	if op == "in" {
		selectedValues, ok := selected.([]interface{})
		if !ok {
			selectedValues = []interface{}{selected}
		}
		if len(selectedValues) == 0 {
			return nil, fmt.Errorf("filter %q requires at least one selected value", definition.Key)
		}
		for _, value := range selectedValues {
			if !containsJSONValue(allowed, value) {
				return nil, fmt.Errorf("value %q is not enabled for filter %q", value, definition.Key)
			}
		}
		return selectedValues, nil
	}
	if _, isArray := selected.([]interface{}); isArray {
		return nil, fmt.Errorf("filter %q requires one selected value", definition.Key)
	}
	if !containsJSONValue(allowed, selected) {
		return nil, fmt.Errorf("value %q is not enabled for filter %q", selected, definition.Key)
	}
	return selected, nil
}

func containsJSONValue(values []interface{}, target interface{}) bool {
	for _, value := range values {
		switch expected := value.(type) {
		case float64:
			actual, ok := target.(float64)
			if ok && expected == actual {
				return true
			}
		case string:
			actual, ok := target.(string)
			if ok && expected == actual {
				return true
			}
		case bool:
			actual, ok := target.(bool)
			if ok && expected == actual {
				return true
			}
		}
	}
	return false
}

func (r *Repository) catalogField(ctx context.Context, key string) (v0CatalogField, error) {
	var field v0CatalogField
	err := r.db.Pool.QueryRow(ctx, `SELECT source_mapping->>'column', allowed_operators, data_type, enum_values FROM segment_field_catalog_v0 WHERE field_key=$1 AND is_active=TRUE`, key).Scan(&field.Column, &field.AllowedOperators, &field.DataType, &field.EnumValues)
	if err != nil {
		return field, fmt.Errorf("unknown or inactive v0 filter field %q", key)
	}
	if !safeCookbookColumn.MatchString(field.Column) {
		return field, fmt.Errorf("invalid source mapping for filter %q", key)
	}
	return field, nil
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}
func validateCatalogValue(field v0CatalogField, value interface{}) error {
	if len(field.EnumValues) > 0 && !contains(field.EnumValues, fmt.Sprint(value)) {
		return fmt.Errorf("value %q is not allowed", value)
	}
	switch field.DataType {
	case "BOOLEAN":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("filter %q requires a boolean value", field.Column)
		}
	case "NUMBER", "INTEGER":
		switch value.(type) {
		case float64, float32, int, int64, json.Number:
		default:
			return fmt.Errorf("filter %q requires a numeric value", field.Column)
		}
	case "STRING":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("filter %q requires a string value", field.Column)
		}
	}
	return nil
}
