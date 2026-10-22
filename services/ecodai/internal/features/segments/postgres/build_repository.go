package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) createBuildConfirm(ctx context.Context, in model.RequestInput, userID, key string) (*model.Request, error) {
	if key != "" {
		var existing string
		err := r.db.Pool.QueryRow(ctx, `SELECT request_id FROM segment_requests_v0
			WHERE requested_by=$1 AND idempotency_key=$2`, userID, key).Scan(&existing)
		if err == nil {
			return r.GetRequest(ctx, existing)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	evaluator := NewBuildEvaluator(r.db)
	buildID, sourceDate, err := evaluator.current(ctx)
	if err != nil {
		return nil, err
	}
	if _, _, err := evaluator.CandidateSQL(ctx, buildID, sourceDate, in); err != nil {
		return nil, err
	}
	versions := map[string]int{}
	ids := []string{}
	if in.Base != "" && in.Base != "all_members" {
		ids = append(ids, in.Base)
	}
	for _, selection := range in.Segments {
		ids = append(ids, selection.ID)
	}
	for _, id := range ids {
		if _, exists := versions[id]; exists {
			continue
		}
		var version int
		if err := r.db.Pool.QueryRow(ctx, `SELECT definition_version FROM segments_v0 WHERE id=$1`, id).Scan(&version); err != nil {
			return nil, err
		}
		versions[id] = version
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	versionJSON, err := json.Marshal(versions)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	requestID := "segreq_" + uuid.NewString()
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var inserted string
	err = tx.QueryRow(ctx, `INSERT INTO segment_requests_v0
		(id,request_id,requested_by,mode,segments_payload,status,idempotency_key,
		 snapshot_date,build_id,org_id,definition_versions)
		VALUES ($1,$2,$3,'confirm',$4,'INIT',$5,$6,$7,'ALL',$8)
		ON CONFLICT (requested_by,idempotency_key) DO NOTHING RETURNING request_id`,
		id, requestID, userID, payload, nullableString(key), sourceDate, buildID, versionJSON).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) && key != "" {
		_ = tx.Rollback(ctx)
		var existing string
		if err := r.db.Pool.QueryRow(ctx, `SELECT request_id FROM segment_requests_v0
			WHERE requested_by=$1 AND idempotency_key=$2`, userID, key).Scan(&existing); err != nil {
			return nil, err
		}
		return r.GetRequest(ctx, existing)
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO segment_request_jobs_v0 (segment_request_id) VALUES ($1)`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO recipes_v0
		(segment_request_id,segments_payload,additional_rules,created_by,build_id,org_id)
		VALUES ($1,$2,'[]'::jsonb,$3,$4,'ALL')`, id, payload, userID, buildID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetRequest(ctx, inserted)
}

func (r *Repository) PinnedBuild(ctx context.Context, requestID uuid.UUID) (string, time.Time, model.RequestInput, error) {
	var buildID string
	var sourceDate time.Time
	var payload []byte
	err := r.db.Pool.QueryRow(ctx, `SELECT r.build_id,b.source_date,r.segments_payload
		FROM segment_requests_v0 r JOIN segment_build_v0 b ON b.build_id=r.build_id
		WHERE r.id=$1 AND b.status='READY'`, requestID).Scan(&buildID, &sourceDate, &payload)
	if err != nil {
		return "", time.Time{}, model.RequestInput{}, err
	}
	var in model.RequestInput
	if err := json.Unmarshal(payload, &in); err != nil {
		return "", time.Time{}, model.RequestInput{}, err
	}
	return buildID, sourceDate, in, nil
}
