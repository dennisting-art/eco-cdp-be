package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"os"
	"time"

	"github.com/google/uuid"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
)

func (w *Worker) processBuild(ctx context.Context, j job, publicID, buildID string) error {
	actualBuild, sourceDate, input, err := w.repo.PinnedBuild(ctx, j.requestID)
	if err != nil {
		return err
	}
	if actualBuild != buildID {
		return fmt.Errorf("pinned build changed for request %s", publicID)
	}
	var rawVersions []byte
	if err := w.db.Pool.QueryRow(ctx, `SELECT definition_versions FROM segment_requests_v0 WHERE id=$1`, j.requestID).Scan(&rawVersions); err != nil {
		return err
	}
	versions := map[string]int{}
	if err := json.Unmarshal(rawVersions, &versions); err != nil {
		return err
	}
	for id, pinnedVersion := range versions {
		var currentVersion int
		if err := w.db.Pool.QueryRow(ctx, `SELECT definition_version FROM segments_v0 WHERE id=$1`, id).Scan(&currentVersion); err != nil {
			return err
		}
		if currentVersion != pinnedVersion {
			return fmt.Errorf("definition %s changed after confirmation", id)
		}
	}
	evaluator := postgres.NewBuildEvaluator(w.db)
	query, args, err := evaluator.CandidateSQL(ctx, buildID, sourceDate, input)
	if err != nil {
		return err
	}
	if err := w.stage(ctx, j, "BUILDING"); err != nil {
		return err
	}
	file, err := os.CreateTemp("", "segments-build-*.csv")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"member_id"}); err != nil {
		return err
	}
	rows, err := w.db.Pool.Query(ctx, `SELECT candidates.member_id,m.build_consent,c.consent,c.refreshed_at
		FROM (`+query+`) candidates
		JOIN member_facts_v0 m ON m.build_id=$1 AND m.member_id=candidates.member_id
		LEFT JOIN member_current_consent_v0 c ON c.member_id=candidates.member_id
		ORDER BY candidates.member_id`, args...)
	if err != nil {
		return err
	}
	var total, buildConsented, exported int64
	checkedAt := time.Now().UTC()
	for rows.Next() {
		var id string
		var buildConsent bool
		var currentConsent *bool
		var refreshed *time.Time
		if err := rows.Scan(&id, &buildConsent, &currentConsent, &refreshed); err != nil {
			rows.Close()
			return err
		}
		total++
		if currentConsent == nil || refreshed == nil || checkedAt.Sub(*refreshed) > 24*time.Hour {
			rows.Close()
			return fmt.Errorf("current consent is missing or stale for member %s", id)
		}
		if buildConsent {
			buildConsented++
		}
		if buildConsent && *currentConsent {
			if err := writer.Write([]string{id}); err != nil {
				rows.Close()
				return err
			}
			exported++
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := w.stage(ctx, j, "UPLOADING"); err != nil {
		return err
	}
	object, err := w.exporter.Upload(ctx, publicID, path)
	if err != nil {
		return err
	}
	if err := w.stage(ctx, j, "VERIFYING"); err != nil {
		return err
	}
	return w.readyBuild(ctx, j, input, total, buildConsented, exported, checkedAt, object)
}

func (w *Worker) readyBuild(ctx context.Context, j job, input model.RequestInput,
	total, buildConsented, exported int64, checkedAt time.Time, object ExportedObject) error {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var recipeID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM recipes_v0 WHERE segment_request_id=$1`, j.requestID).Scan(&recipeID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE recipes_v0 SET member_count=$2,computed_at=now() WHERE id=$1`, recipeID, exported); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE segment_requests_v0 r
		SET status='READY',progress_stage='COMPLETE',total_count=$3,
		build_consent_count=$4,consent_count=$5,consent_checked_at=$6,
		bucket=$7,bytes_uploaded=$8,files_completed=1,completed_at=now()
		FROM segment_request_jobs_v0 j
		WHERE r.id=$1 AND j.id=$2 AND j.segment_request_id=r.id
		AND j.locked_by=$9 AND j.status='PROCESSING' AND j.lease_until>now()`,
		j.requestID, j.id, total, buildConsented, exported, checkedAt, object.URI, object.Bytes, w.owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("segment job lease lost before completion")
	}
	if _, err := tx.Exec(ctx, `UPDATE segment_request_jobs_v0
		SET status='DONE',lease_until=NULL,updated_at=now() WHERE id=$1`, j.id); err != nil {
		return err
	}
	for _, selection := range input.Segments {
		if _, err := tx.Exec(ctx, `INSERT INTO segment_usage_v0(segment_id,recipe_id,event_type)
			VALUES($1,$2,'REQUEST_CONFIRMED')`, selection.ID, recipeID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
