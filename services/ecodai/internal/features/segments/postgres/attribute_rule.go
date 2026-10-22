package postgres

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// attributeRuleCondition accepts only the fields and comparisons used by the
// reviewed attribute catalogue. Values are always bound query parameters.
func attributeRuleCondition(raw json.RawMessage, args *[]interface{}, next *int) (string, error) {
	var rule struct {
		Key      string  `json:"key"`
		Equal    *string `json:"equal"`
		NotEqual *string `json:"not_equal"`
		Min      *int    `json:"min"`
		Max      *int    `json:"max"`
	}
	if err := json.Unmarshal(raw, &rule); err != nil {
		return "", fmt.Errorf("decode attribute rule: %w", err)
	}
	var expression, guard string
	switch rule.Key {
	case "cc_active_flag":
		expression = "m.cc_active_flag"
	case "nationality_code":
		expression = "m.nationality_code"
	case "gender":
		expression = "m.gender"
	case "marital_status":
		expression = "m.marital_status"
	case "race":
		expression = "m.race"
	case "age_years":
		expression = "EXTRACT(YEAR FROM AGE($2::date,m.birth_date))::int"
		guard = "m.birth_date <= $2::date"
	case "birth_year":
		expression = "EXTRACT(YEAR FROM m.birth_date)::int"
		guard = "m.birth_date <= $2::date"
	case "tenure_days":
		expression = "($2::date - m.membership_application_date)"
		guard = "m.membership_application_date <= $2::date"
	default:
		return "", fmt.Errorf("unsupported attribute field %q", rule.Key)
	}

	conditions := []string{}
	if guard != "" {
		conditions = append(conditions, guard)
	}
	if rule.Key == "cc_active_flag" || rule.Key == "nationality_code" || rule.Key == "gender" || rule.Key == "marital_status" || rule.Key == "race" {
		if rule.Min != nil || rule.Max != nil || (rule.Equal == nil) == (rule.NotEqual == nil) {
			return "", fmt.Errorf("invalid equality rule for %q", rule.Key)
		}
		value, operator := rule.Equal, "="
		if value == nil {
			value, operator = rule.NotEqual, "<>"
		}
		var parameter interface{} = *value
		if rule.Key == "cc_active_flag" {
			boolean, err := strconv.ParseBool(*value)
			if err != nil {
				return "", fmt.Errorf("invalid credit-card rule: %w", err)
			}
			parameter = boolean
		}
		conditions = append(conditions, fmt.Sprintf("%s %s $%d", expression, operator, *next))
		*args = append(*args, parameter)
		*next++
	} else {
		if rule.Equal != nil || rule.NotEqual != nil || (rule.Min == nil && rule.Max == nil) || (rule.Min != nil && rule.Max != nil && *rule.Min > *rule.Max) {
			return "", fmt.Errorf("invalid range rule for %q", rule.Key)
		}
		if rule.Min != nil {
			conditions = append(conditions, fmt.Sprintf("%s >= $%d", expression, *next))
			*args = append(*args, *rule.Min)
			*next++
		}
		if rule.Max != nil {
			conditions = append(conditions, fmt.Sprintf("%s <= $%d", expression, *next))
			*args = append(*args, *rule.Max)
			*next++
		}
	}
	return "(" + strings.Join(conditions, " AND ") + ")", nil
}
