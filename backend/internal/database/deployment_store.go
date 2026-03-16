package database

import (
	"context"
	"fmt"
	"time"

	"instantdeploy/backend/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DeploymentStore struct {
	pool *pgxpool.Pool
}

func NewDeploymentStore(pool *pgxpool.Pool) *DeploymentStore {
	if pool == nil {
		return nil
	}
	return &DeploymentStore{pool: pool}
}

func (s *DeploymentStore) EnsureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const schema = `
CREATE TABLE IF NOT EXISTS deployments (
    id TEXT PRIMARY KEY,
    repository TEXT NOT NULL,
    branch TEXT NOT NULL,
    status TEXT NOT NULL,
    url TEXT NOT NULL,
    local_url TEXT NOT NULL DEFAULT '',
    repo_url TEXT NOT NULL DEFAULT '',
    image TEXT NOT NULL DEFAULT '',
    container TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);

ALTER TABLE deployments ADD COLUMN IF NOT EXISTS local_url TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS repo_url TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS container TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS error TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;

CREATE TABLE IF NOT EXISTS deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    time TIMESTAMP NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_id ON deployment_logs(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_time ON deployment_logs(time);
`

	_, err := s.pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}

func (s *DeploymentStore) ListDeployments() ([]models.Deployment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx, `
SELECT id, repository, branch, status, url, local_url, repo_url, image, container, error, created_at
FROM deployments
ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Deployment, 0)
	for rows.Next() {
		var d models.Deployment
		if scanErr := rows.Scan(
			&d.ID,
			&d.Repository,
			&d.Branch,
			&d.Status,
			&d.URL,
			&d.LocalURL,
			&d.RepoURL,
			&d.Image,
			&d.Container,
			&d.Error,
			&d.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *DeploymentStore) ListLogsByDeployment() (map[string][]models.DeploymentLog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx, `
SELECT deployment_id, time, level, message
FROM deployment_logs
ORDER BY time ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logsByDeployment := make(map[string][]models.DeploymentLog)
	for rows.Next() {
		var deploymentID string
		var log models.DeploymentLog
		if scanErr := rows.Scan(&deploymentID, &log.Time, &log.Level, &log.Message); scanErr != nil {
			return nil, scanErr
		}
		logsByDeployment[deploymentID] = append(logsByDeployment[deploymentID], log)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return logsByDeployment, nil
}

func (s *DeploymentStore) UpsertDeployment(d models.Deployment) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, `
INSERT INTO deployments (id, repository, branch, status, url, local_url, repo_url, image, container, error, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (id) DO UPDATE
SET repository = EXCLUDED.repository,
    branch = EXCLUDED.branch,
    status = EXCLUDED.status,
    url = EXCLUDED.url,
    local_url = EXCLUDED.local_url,
    repo_url = EXCLUDED.repo_url,
    image = EXCLUDED.image,
    container = EXCLUDED.container,
    error = EXCLUDED.error`,
		d.ID,
		d.Repository,
		d.Branch,
		d.Status,
		d.URL,
		d.LocalURL,
		d.RepoURL,
		d.Image,
		d.Container,
		d.Error,
		d.CreatedAt,
	)
	return err
}

func (s *DeploymentStore) AppendLog(deploymentID string, log models.DeploymentLog) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, `
INSERT INTO deployment_logs (deployment_id, time, level, message)
VALUES ($1, $2, $3, $4)`, deploymentID, log.Time, log.Level, log.Message)
	return err
}

func (s *DeploymentStore) GetDeployment(id string) (models.Deployment, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var d models.Deployment
	err := s.pool.QueryRow(ctx, `
SELECT id, repository, branch, status, url, local_url, repo_url, image, container, error, created_at
FROM deployments WHERE id = $1`, id).Scan(
		&d.ID, &d.Repository, &d.Branch, &d.Status, &d.URL,
		&d.LocalURL, &d.RepoURL, &d.Image, &d.Container, &d.Error, &d.CreatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return models.Deployment{}, false, nil
		}
		return models.Deployment{}, false, err
	}
	return d, true, nil
}

func (s *DeploymentStore) DeleteDeployment(deploymentID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, `DELETE FROM deployments WHERE id = $1`, deploymentID)
	return err
}
