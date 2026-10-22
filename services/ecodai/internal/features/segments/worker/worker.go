package worker

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
	"time"
)

const maxAttempts = 3

type Worker struct {
	db       *database.PostgresDB
	repo     *postgres.Repository
	exporter Exporter
	owner    string
}
type job struct {
	id        uuid.UUID
	requestID uuid.UUID
	attempts  int
}

func NewWorker(db *database.PostgresDB, exporter Exporter) *Worker {
	return &Worker{db: db, repo: postgres.NewRepository(db), exporter: exporter, owner: uuid.NewString()}
}
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		jobCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		done, err := w.ProcessNext(jobCtx)
		cancel()
		if err != nil {
			logger.Error(ctx, "segment export job failed", "error", err)
		}
		if done {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w *Worker) claim(ctx context.Context) (job, bool, error) {
	var j job
	err := w.db.Pool.QueryRow(ctx, `WITH candidate AS (
 SELECT j.id FROM segment_request_jobs_v0 j JOIN segment_requests_v0 r ON r.id=j.segment_request_id
 WHERE ((j.status='PENDING' AND j.next_run_at<=now()) OR
        (j.status='PROCESSING' AND j.lease_until<now())) AND r.status IN ('INIT','PROCESSING')
 ORDER BY j.created_at FOR UPDATE OF j SKIP LOCKED LIMIT 1)
 UPDATE segment_request_jobs_v0 j SET status='PROCESSING',attempt_count=j.attempt_count+1,
 locked_at=now(),locked_by=$1,lease_until=now()+interval '2 minutes',updated_at=now()
 FROM candidate c WHERE j.id=c.id RETURNING j.id,j.segment_request_id,j.attempt_count`, w.owner).Scan(&j.id, &j.requestID, &j.attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, false, nil
	}
	return j, err == nil, err
}
func (w *Worker) ProcessNext(ctx context.Context) (bool, error) {
	if w.exporter == nil {
		return false, fmt.Errorf("segment exporter is not configured")
	}
	j, ok, err := w.claim(ctx)
	if err != nil || !ok {
		return ok, err
	}
	if err := w.stage(ctx, j, "EVALUATING"); err != nil {
		return true, err
	}
	keepCtx, stop := context.WithCancel(ctx)
	defer stop()
	go w.renew(keepCtx, j)
	err = w.process(ctx, j)
	if err == nil {
		return true, nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return true, w.retry(cleanupCtx, j, err)
}
func (w *Worker) renew(ctx context.Context, j job) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			renewCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := w.db.Pool.Exec(renewCtx, `UPDATE segment_request_jobs_v0 SET lease_until=now()+interval '2 minutes' WHERE id=$1 AND locked_by=$2 AND status='PROCESSING'`, j.id, w.owner)
			cancel()
			if err != nil {
				logger.Warn(ctx, "segment job lease renewal failed", "error", err)
			}
		}
	}
}
func (w *Worker) stage(ctx context.Context, j job, stage string) error {
	tag, err := w.db.Pool.Exec(ctx, `UPDATE segment_requests_v0 r SET status='PROCESSING',progress_stage=$3
 FROM segment_request_jobs_v0 j WHERE r.id=$1 AND j.id=$2 AND j.segment_request_id=r.id AND j.locked_by=$4 AND j.status='PROCESSING' AND j.lease_until>now()`, j.requestID, j.id, stage, w.owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("segment job lease lost")
	}
	return nil
}
func (w *Worker) retry(ctx context.Context, j job, cause error) error {
	terminal := j.attempts >= maxAttempts
	status, requestStatus, stage, next := "PENDING", "INIT", "QUEUED", time.Now().Add(time.Duration(j.attempts*j.attempts)*time.Minute)
	if terminal {
		status, requestStatus, stage = "FAILED", "FAILED", "FAILED"
	}
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return errors.Join(cause, err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE segment_request_jobs_v0 SET status=$3,next_run_at=$4,last_error=$5,
 lease_until=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND locked_by=$2 AND status='PROCESSING'`, j.id, w.owner, status, next, cause.Error())
	if err != nil {
		return errors.Join(cause, err)
	}
	if tag.RowsAffected() != 1 {
		return cause
	}
	_, err = tx.Exec(ctx, `UPDATE segment_requests_v0 SET status=$2,progress_stage=$3,error_code=CASE WHEN $4 THEN 'CONFIRM_FAILED' ELSE NULL END,
 completed_at=CASE WHEN $4 THEN now() ELSE NULL END WHERE id=$1`, j.requestID, requestStatus, stage, terminal)
	if err != nil {
		return errors.Join(cause, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}
