-- InstantDeploy Database Initialization
-- This runs on first postgres container start via docker-entrypoint-initdb.d

-- Users table
CREATE TABLE IF NOT EXISTS users (
    username TEXT PRIMARY KEY,
    email TEXT NOT NULL DEFAULT '',
    password_hash BYTEA NOT NULL DEFAULT '\x',
    role TEXT NOT NULL DEFAULT 'developer',
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    verification_code TEXT NOT NULL DEFAULT '',
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique_lower ON users (lower(email));

-- Deployments table
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Deployment logs table
CREATE TABLE IF NOT EXISTS deployment_logs (
    id BIGSERIAL PRIMARY KEY,
    deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    time TIMESTAMP NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_id ON deployment_logs(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_time ON deployment_logs(time);
