package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"os"
	"sort"
	"strings"
	"time"
)

func (w *Worker) process(ctx context.Context, j job) error {
	var publicID string
	var payload []byte
	var rawDates []byte
	var buildID *string
	err := w.db.Pool.QueryRow(ctx, `SELECT request_id,segments_payload,snapshot_dates,build_id
		FROM segment_requests_v0 WHERE id=$1`, j.requestID).Scan(&publicID, &payload, &rawDates, &buildID)
	if err != nil {
		return err
	}
	if buildID != nil {
		return w.processBuild(ctx, j, publicID, *buildID)
	}
	var selections []model.SegmentSelection
	if err = json.Unmarshal(payload, &selections); err != nil {
		return err
	}
	var dates map[string]string
	if err = json.Unmarshal(rawDates, &dates); err != nil {
		return err
	}
	candidates, err := w.evaluate(ctx, selections, dates)
	if err != nil {
		return err
	}
	total := int64(len(candidates))
	approved, err := w.consent(ctx, candidates)
	if err != nil {
		return err
	}
	if err = w.stage(ctx, j, "BUILDING"); err != nil {
		return err
	}
	f, err := os.CreateTemp("", "segments-v0-*.csv")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	csvWriter := csv.NewWriter(f)
	if err = csvWriter.Write([]string{"member_id"}); err == nil {
		for _, id := range approved {
			if err = csvWriter.Write([]string{id}); err != nil {
				break
			}
		}
	}
	csvWriter.Flush()
	if err == nil {
		err = csvWriter.Error()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = w.stage(ctx, j, "UPLOADING"); err != nil {
		return err
	}
	object, err := w.exporter.Upload(ctx, publicID, path)
	if err != nil {
		return err
	}
	if err = w.stage(ctx, j, "VERIFYING"); err != nil {
		return err
	}
	return w.ready(ctx, j, selections, total, approved, object)
}
func (w *Worker) evaluate(ctx context.Context, selections []model.SegmentSelection, dates map[string]string) ([]string, error) {
	var result map[string]struct{}
	for i, s := range selections {
		date, ok := dates[s.ID]
		if !ok {
			return nil, fmt.Errorf("missing pinned snapshot for segment %s", s.ID)
		}
		pinned, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, err
		}
		ids, err := w.repo.MatchMembersAt(ctx, s, false, &pinned)
		if err != nil {
			return nil, err
		}
		current := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			current[id] = struct{}{}
		}
		if i == 0 {
			result = current
			continue
		}
		if strings.EqualFold(selections[i-1].Operation, "AND") {
			for id := range result {
				if _, ok := current[id]; !ok {
					delete(result, id)
				}
			}
		} else {
			for id := range current {
				result[id] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(result))
	for id := range result {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
func (w *Worker) consent(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}
	approved := make([]string, 0, len(ids))
	for start := 0; start < len(ids); start += 1000 {
		end := start + 1000
		if end > len(ids) {
			end = len(ids)
		}
		rows, err := w.db.Pool.Query(ctx, `SELECT member_id FROM member_cookbook_v0 WHERE member_id=ANY($1) AND consent_flag=TRUE`, ids[start:end])
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			approved = append(approved, id)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(approved)
	return approved, nil
}
func (w *Worker) ready(ctx context.Context, j job, selections []model.SegmentSelection, total int64, approved []string, obj ExportedObject) error {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var recipeID uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM recipes_v0 WHERE segment_request_id=$1`, j.requestID).Scan(&recipeID); err != nil {
		return err
	}
	for start := 0; start < len(approved); start += 1000 {
		end := start + 1000
		if end > len(approved) {
			end = len(approved)
		}
		_, err = tx.Exec(ctx, `INSERT INTO recipe_members_v0(recipe_id,member_id,computed_at) SELECT $1,unnest($2::text[]),now() ON CONFLICT DO NOTHING`, recipeID, approved[start:end])
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE recipes_v0 SET member_count=$2,computed_at=now() WHERE id=$1`, recipeID, len(approved)); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE segment_requests_v0 r SET status='READY',progress_stage='COMPLETE',total_count=$3,consent_count=$4,
 bucket=$5,bytes_uploaded=$6,files_completed=1,completed_at=now() FROM segment_request_jobs_v0 j
 WHERE r.id=$1 AND j.id=$2 AND j.segment_request_id=r.id AND j.locked_by=$7 AND j.status='PROCESSING' AND j.lease_until>now()`, j.requestID, j.id, total, len(approved), obj.URI, obj.Bytes, w.owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("segment job lease lost before completion")
	}
	if _, err = tx.Exec(ctx, `UPDATE segment_request_jobs_v0 SET status='DONE',lease_until=NULL,updated_at=now() WHERE id=$1`, j.id); err != nil {
		return err
	}
	for _, s := range selections {
		if _, err = tx.Exec(ctx, `INSERT INTO segment_usage_v0(segment_id,recipe_id,event_type) VALUES($1,$2,'REQUEST_CONFIRMED')`, s.ID, recipeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
