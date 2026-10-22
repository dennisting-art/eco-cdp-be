package service

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"time"
)

func buildUniverseMatrix(s model.UniverseSnapshot) (*model.UniverseViewMatrix, *universeChartIssue) {
	if reason := s.InvalidCharts["member_matrix_activity"]; reason != "" {
		return nil, invalidChart(reason)
	}
	if s.MatrixDate.IsZero() {
		return nil, missingChart("missing_matrix_date")
	}
	configured := map[string]bool{}
	for _, scope := range s.Scopes {
		configured[scope.Code] = true
	}
	participants := map[string]bool{}
	for code := range s.MatrixTotals {
		participants[code] = true
	}
	cells := map[string]model.UniverseCell{}
	for _, cell := range s.Cells {
		participants[cell.Primary], participants[cell.Related] = true, true
		if cell.Count < 0 || !validUniverseRatio(cell.Ratio) {
			return nil, invalidChart("invalid_matrix_cell")
		}
		key := cell.Primary + "\x00" + cell.Related
		if _, exists := cells[key]; exists {
			return nil, invalidChart("duplicate_matrix_cell")
		}
		cells[key] = cell
	}
	if len(participants) == 0 {
		return nil, missingChart("missing_matrix_entities")
	}
	for code := range participants {
		if !configured[code] {
			return nil, invalidChart("unknown_matrix_entity", "entity", code)
		}
	}
	ordered := make([]string, 0, len(participants))
	for _, scope := range s.Scopes {
		if participants[scope.Code] {
			ordered = append(ordered, scope.Code)
		}
	}
	for _, primary := range ordered {
		if total := s.MatrixTotals[primary]; total != nil && *total < 0 {
			return nil, invalidChart("negative_primary_total", "primary_entity", primary)
		}
	}
	if len(cells) != len(ordered)*len(ordered) {
		return nil, missingChart("missing_relationship_cells", "expected_cells", len(ordered)*len(ordered), "actual_cells", len(cells))
	}
	out := &model.UniverseViewMatrix{Columns: append([]string{}, ordered...), Rows: []model.UniverseViewMatrixRow{}}
	for _, primary := range ordered {
		total := s.MatrixTotals[primary]
		if total == nil {
			return nil, missingChart("missing_primary_total", "primary_entity", primary)
		}
		row := model.UniverseViewMatrixRow{PrimaryEntity: model.UniverseViewMetric{Label: primary, Count: total}, Relationships: []model.UniverseViewMetric{}}
		for _, related := range ordered {
			cell := cells[primary+"\x00"+related]
			count, ratio := cell.Count, cell.Ratio
			row.Relationships = append(row.Relationships, model.UniverseViewMetric{Label: related, Count: &count, Percent: &ratio})
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func applyUniverseMatrix(out *model.UniverseView, s model.UniverseSnapshot, record func(string, string, time.Time, *universeChartIssue)) {
	if s.MatrixEnabled {
		matrixIssue := universeSnapshotIssue(s.MatrixDate, s.Range.End)
		var matrix *model.UniverseViewMatrix
		if matrixIssue == nil {
			matrix, matrixIssue = buildUniverseMatrix(s)
		}
		if matrixIssue == nil {
			out.Group.MemberMatrixActivity = matrix
		}
		record("member_matrix_activity", "", s.MatrixDate, matrixIssue)
	} else {
		out.AsOfDates.Group["member_matrix_activity"] = nil
	}
}
