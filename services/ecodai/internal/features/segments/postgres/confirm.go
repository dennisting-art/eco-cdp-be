package postgres

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"time"
)

func (r *Repository) CreateConfirm(ctx context.Context, in model.RequestInput, userID string, key string) (*model.Request, error) {
	if r.versioned {
		return r.createBuildConfirm(ctx, in, userID, key)
	}
	if key != "" {
		var existingID string
		err := r.db.Pool.QueryRow(ctx, `SELECT request_id FROM segment_requests_v0 WHERE requested_by=$1 AND idempotency_key=$2`, userID, key).Scan(&existingID)
		if err == nil {
			return r.GetRequest(ctx, existingID)
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}
	}
	var snapshot *time.Time
	dates := map[string]string{}
	for _, s := range in.Segments {
		if _, ok := dates[s.ID]; !ok {
			var current *time.Time
			if err := r.db.Pool.QueryRow(ctx, `SELECT max(sm.computed_date) FROM segment_members_v0 sm JOIN segments_v0 s ON s.id=$1 AND s.is_visible=TRUE WHERE sm.segment_id=COALESCE(s.base_segment_id,s.id)`, s.ID).Scan(&current); err != nil {
				return nil, err
			}
			if current == nil {
				dates[s.ID] = "0001-01-01"
			} else {
				dates[s.ID] = current.Format("2006-01-02")
				if snapshot == nil || current.After(*snapshot) {
					snapshot = current
				}
			}
		}
		pinned, err := time.Parse("2006-01-02", dates[s.ID])
		if err != nil {
			return nil, err
		}
		if _, err := r.MatchMembersAt(ctx, s, false, &pinned); err != nil {
			return nil, apierrors.Wrap("10001", "invalid request", 400, err)
		}
	}
	snapshotDates, err := json.Marshal(dates)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(in.Segments)
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
	q := `INSERT INTO segment_requests_v0 (id,request_id,requested_by,mode,segments_payload,status,idempotency_key,snapshot_date,snapshot_dates) VALUES ($1,$2,$3,'confirm',$4,'INIT',$5,$6,$7) ON CONFLICT (requested_by,idempotency_key) DO NOTHING RETURNING request_id`
	var insertedID string
	if err = tx.QueryRow(ctx, q, id, requestID, userID, payload, nullableString(key), snapshot, snapshotDates).Scan(&insertedID); err != nil {
		if err == pgx.ErrNoRows && key != "" {
			_ = tx.Rollback(ctx)
			var existing string
			if e := r.db.Pool.QueryRow(ctx, `SELECT request_id FROM segment_requests_v0 WHERE requested_by=$1 AND idempotency_key=$2`, userID, key).Scan(&existing); e != nil {
				return nil, e
			}
			return r.GetRequest(ctx, existing)
		}
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO segment_request_jobs_v0 (segment_request_id) VALUES ($1)", id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO recipes_v0 (segment_request_id,segments_payload,additional_rules,created_by) VALUES ($1,$2,'[]'::jsonb,$3)", id, payload, userID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetRequest(ctx, requestID)
}
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
