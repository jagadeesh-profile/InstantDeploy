-- InstantDeploy — initial schema
-- Runs once when postgres container is first created

CREATE TABLE IF NOT EXISTS users (
  username          TEXT PRIMARY KEY,
  email             TEXT NOT NULL UNIQUE,
  password_hash     BYTEA NOT NULL,
  role              TEXT NOT NULL DEFAULT 'developer',
  verified          BOOLEAN NOT NULL DEFAULT FALSE,
  verification_code TEXT,
  failed_attempts   INTEGER NOT NULL DEFAULT 0,
  locked_until      TIMESTAMP,
  created_at        TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deployments (
  id          TEXT PRIMARY KEY,
  repository  TEXT NOT NULL,
  branch      TEXT NOT NULL DEFAULT 'main',
  status      TEXT NOT NULL,
  url         TEXT NOT NULL DEFAULT '',
  local_url   TEXT NOT NULL DEFAULT '',
  repo_url    TEXT NOT NULL DEFAULT '',
  image       TEXT NOT NULL DEFAULT '',
  container   TEXT NOT NULL DEFAULT '',
  error       TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deployment_logs (
  id            BIGSERIAL PRIMARY KEY,
  deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
  time          TIMESTAMP NOT NULL,
  level         TEXT NOT NULL,
  message       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_dep_id ON deployment_logs(deployment_id, time);
