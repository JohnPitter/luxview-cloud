package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/pkg/crypto"
)

type ActionRepo struct {
	db            *DB
	encryptionKey []byte
}

func NewActionRepo(db *DB, encryptionKey []byte) *ActionRepo {
	return &ActionRepo{db: db, encryptionKey: encryptionKey}
}

func (r *ActionRepo) CreateRun(ctx context.Context, run *model.ActionRun, jobs []model.ActionJob, stepsByJob map[string][]model.ActionStep) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create action run: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO action_runs (app_id, workflow, workflow_path, trigger, branch, commit_sha, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		run.AppID, run.Workflow, run.WorkflowPath, run.Trigger, run.Branch, run.CommitSHA, run.Status,
	).Scan(&run.ID, &run.CreatedAt)
	if err != nil {
		return fmt.Errorf("create action run: %w", err)
	}

	for i := range jobs {
		jobs[i].RunID = run.ID
		err = tx.QueryRow(ctx,
			`INSERT INTO action_jobs (run_id, name, image, status)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id, created_at`,
			jobs[i].RunID, jobs[i].Name, jobs[i].Image, jobs[i].Status,
		).Scan(&jobs[i].ID, &jobs[i].CreatedAt)
		if err != nil {
			return fmt.Errorf("create action job: %w", err)
		}

		for _, step := range stepsByJob[jobs[i].Name] {
			inputsJSON, err := json.Marshal(step.Inputs)
			if err != nil {
				return fmt.Errorf("marshal action step inputs: %w", err)
			}
			_, err = tx.Exec(ctx,
				`INSERT INTO action_steps (job_id, name, kind, command, uses, inputs, status, position)
				 VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)`,
				jobs[i].ID, step.Name, step.Kind, step.Command, step.Uses, string(inputsJSON), step.Status, step.Position,
			)
			if err != nil {
				return fmt.Errorf("create action step: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create action run: %w", err)
	}
	return nil
}

func (r *ActionRepo) ListRunsByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]model.ActionRun, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_runs WHERE app_id = $1`, appID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, app_id, workflow, workflow_path, trigger, branch, commit_sha, status, created_at, started_at, finished_at
		 FROM action_runs WHERE app_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, appID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var runs []model.ActionRun
	for rows.Next() {
		var run model.ActionRun
		if err := rows.Scan(&run.ID, &run.AppID, &run.Workflow, &run.WorkflowPath, &run.Trigger, &run.Branch, &run.CommitSHA, &run.Status, &run.CreatedAt, &run.StartedAt, &run.FinishedAt); err != nil {
			return nil, 0, err
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

func (r *ActionRepo) FindRunDetail(ctx context.Context, runID uuid.UUID) (*model.ActionRunDetail, error) {
	var detail model.ActionRunDetail
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, workflow, workflow_path, trigger, branch, commit_sha, status, created_at, started_at, finished_at
		 FROM action_runs WHERE id = $1`, runID,
	).Scan(&detail.Run.ID, &detail.Run.AppID, &detail.Run.Workflow, &detail.Run.WorkflowPath, &detail.Run.Trigger, &detail.Run.Branch, &detail.Run.CommitSHA, &detail.Run.Status, &detail.Run.CreatedAt, &detail.Run.StartedAt, &detail.Run.FinishedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find action run: %w", err)
	}

	jobs, err := r.listJobs(ctx, runID)
	if err != nil {
		return nil, err
	}
	steps, err := r.listSteps(ctx, runID)
	if err != nil {
		return nil, err
	}
	detail.Jobs = jobs
	detail.Steps = steps
	return &detail, nil
}

func (r *ActionRepo) ClaimNextQueuedRun(ctx context.Context) (*model.ActionRun, error) {
	var run model.ActionRun
	err := r.db.Pool.QueryRow(ctx,
		`UPDATE action_runs
		 SET status = 'running', started_at = NOW()
		 WHERE id = (
		   SELECT id FROM action_runs
		   WHERE status = 'queued'
		   ORDER BY created_at
		   FOR UPDATE SKIP LOCKED
		   LIMIT 1
		 )
		 RETURNING id, app_id, workflow, workflow_path, trigger, branch, commit_sha, status, created_at, started_at, finished_at`,
	).Scan(&run.ID, &run.AppID, &run.Workflow, &run.WorkflowPath, &run.Trigger, &run.Branch, &run.CommitSHA, &run.Status, &run.CreatedAt, &run.StartedAt, &run.FinishedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim action run: %w", err)
	}
	return &run, nil
}

func (r *ActionRepo) ListJobsForRun(ctx context.Context, runID uuid.UUID) ([]model.ActionJob, error) {
	return r.listJobs(ctx, runID)
}

func (r *ActionRepo) ListStepsForJob(ctx context.Context, jobID uuid.UUID) ([]model.ActionStep, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, job_id, name, kind, command, uses, inputs, status, exit_code, log, position, started_at, finished_at
		 FROM action_steps WHERE job_id = $1 ORDER BY position`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []model.ActionStep
	for rows.Next() {
		var step model.ActionStep
		var inputs []byte
		if err := rows.Scan(&step.ID, &step.JobID, &step.Name, &step.Kind, &step.Command, &step.Uses, &inputs, &step.Status, &step.ExitCode, &step.Log, &step.Position, &step.StartedAt, &step.FinishedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(inputs, &step.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal action step inputs: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *ActionRepo) UpdateRunStatus(ctx context.Context, runID uuid.UUID, status model.ActionStatus) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE action_runs
		 SET status = $2, finished_at = CASE WHEN $2 IN ('success', 'failed', 'cancelled') THEN NOW() ELSE finished_at END
		 WHERE id = $1`, runID, status)
	return err
}

func (r *ActionRepo) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status model.ActionStatus) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE action_jobs
		 SET status = $2,
		     started_at = CASE WHEN $2 = 'running' THEN NOW() ELSE started_at END,
		     finished_at = CASE WHEN $2 IN ('success', 'failed', 'cancelled', 'skipped') THEN NOW() ELSE finished_at END
		 WHERE id = $1`, jobID, status)
	return err
}

func (r *ActionRepo) UpdateStepResult(ctx context.Context, stepID uuid.UUID, status model.ActionStatus, exitCode int, log string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE action_steps
		 SET status = $2, exit_code = $3, log = $4,
		     started_at = COALESCE(started_at, NOW()),
		     finished_at = NOW()
		 WHERE id = $1`, stepID, status, exitCode, log)
	return err
}

func (r *ActionRepo) StartStep(ctx context.Context, stepID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE action_steps SET status = 'running', started_at = NOW() WHERE id = $1`, stepID)
	return err
}

func (r *ActionRepo) listJobs(ctx context.Context, runID uuid.UUID) ([]model.ActionJob, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, run_id, name, image, status, created_at, started_at, finished_at
		 FROM action_jobs WHERE run_id = $1 ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.ActionJob
	for rows.Next() {
		var job model.ActionJob
		if err := rows.Scan(&job.ID, &job.RunID, &job.Name, &job.Image, &job.Status, &job.CreatedAt, &job.StartedAt, &job.FinishedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *ActionRepo) listSteps(ctx context.Context, runID uuid.UUID) ([]model.ActionStep, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT s.id, s.job_id, s.name, s.kind, s.command, s.uses, s.inputs, s.status, s.exit_code, s.log, s.position, s.started_at, s.finished_at
		 FROM action_steps s
		 JOIN action_jobs j ON j.id = s.job_id
		 WHERE j.run_id = $1
		 ORDER BY j.created_at, s.position`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []model.ActionStep
	for rows.Next() {
		var step model.ActionStep
		var inputs []byte
		if err := rows.Scan(&step.ID, &step.JobID, &step.Name, &step.Kind, &step.Command, &step.Uses, &inputs, &step.Status, &step.ExitCode, &step.Log, &step.Position, &step.StartedAt, &step.FinishedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(inputs, &step.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal action step inputs: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (r *ActionRepo) UpsertSecret(ctx context.Context, appID uuid.UUID, key, value string) (*model.ActionSecret, error) {
	encrypted, err := crypto.Encrypt(value, r.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt action secret: %w", err)
	}

	var secret model.ActionSecret
	err = r.db.Pool.QueryRow(ctx,
		`INSERT INTO action_secrets (app_id, key, encrypted_value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (app_id, key)
		 DO UPDATE SET encrypted_value = $3, updated_at = NOW()
		 RETURNING id, app_id, key, created_at, updated_at`,
		appID, key, encrypted,
	).Scan(&secret.ID, &secret.AppID, &secret.Key, &secret.CreatedAt, &secret.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert action secret: %w", err)
	}
	return &secret, nil
}

func (r *ActionRepo) ListSecrets(ctx context.Context, appID uuid.UUID) ([]model.ActionSecret, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, app_id, key, created_at, updated_at
		 FROM action_secrets WHERE app_id = $1 ORDER BY key`, appID)
	if err != nil {
		return nil, fmt.Errorf("list action secrets: %w", err)
	}
	defer rows.Close()

	var secrets []model.ActionSecret
	for rows.Next() {
		var secret model.ActionSecret
		if err := rows.Scan(&secret.ID, &secret.AppID, &secret.Key, &secret.CreatedAt, &secret.UpdatedAt); err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	return secrets, rows.Err()
}

func (r *ActionRepo) GetSecretValues(ctx context.Context, appID uuid.UUID) (map[string]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT key, encrypted_value FROM action_secrets WHERE app_id = $1 ORDER BY key`, appID)
	if err != nil {
		return nil, fmt.Errorf("get action secrets: %w", err)
	}
	defer rows.Close()

	secrets := make(map[string]string)
	for rows.Next() {
		var key, encrypted string
		if err := rows.Scan(&key, &encrypted); err != nil {
			return nil, err
		}
		value, err := crypto.Decrypt(encrypted, r.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt action secret %q: %w", key, err)
		}
		secrets[key] = value
	}
	return secrets, rows.Err()
}

func (r *ActionRepo) DeleteSecret(ctx context.Context, appID uuid.UUID, key string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM action_secrets WHERE app_id = $1 AND key = $2`, appID, key)
	if err != nil {
		return fmt.Errorf("delete action secret: %w", err)
	}
	return nil
}

func (r *ActionRepo) UpsertArtifact(ctx context.Context, artifact *model.ActionArtifact) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO action_artifacts (run_id, name, path, size_bytes)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (run_id, name)
		 DO UPDATE SET path = $3, size_bytes = $4, created_at = NOW()
		 RETURNING id, created_at`,
		artifact.RunID, artifact.Name, artifact.Path, artifact.SizeBytes,
	).Scan(&artifact.ID, &artifact.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert action artifact: %w", err)
	}
	return nil
}

func (r *ActionRepo) FindArtifact(ctx context.Context, runID uuid.UUID, name string) (*model.ActionArtifact, error) {
	var artifact model.ActionArtifact
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, run_id, name, path, size_bytes, created_at
		 FROM action_artifacts WHERE run_id = $1 AND name = $2`, runID, name,
	).Scan(&artifact.ID, &artifact.RunID, &artifact.Name, &artifact.Path, &artifact.SizeBytes, &artifact.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find action artifact: %w", err)
	}
	return &artifact, nil
}

func (r *ActionRepo) ListArtifactsByRunID(ctx context.Context, runID uuid.UUID) ([]model.ActionArtifact, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, run_id, name, path, size_bytes, created_at
		 FROM action_artifacts WHERE run_id = $1 ORDER BY created_at DESC`, runID)
	if err != nil {
		return nil, fmt.Errorf("list action artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []model.ActionArtifact
	for rows.Next() {
		var artifact model.ActionArtifact
		if err := rows.Scan(&artifact.ID, &artifact.RunID, &artifact.Name, &artifact.Path, &artifact.SizeBytes, &artifact.CreatedAt); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}
