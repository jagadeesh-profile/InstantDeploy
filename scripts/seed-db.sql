-- Seed database with initial schema and demo data.
-- The Go backend auto-migrates on startup; this file seeds demo rows.

CREATE TABLE IF NOT EXISTS deployments (
  id         TEXT PRIMARY KEY,
  repository TEXT NOT NULL,
  branch     TEXT NOT NULL,
  status     TEXT NOT NULL,
  url        TEXT NOT NULL DEFAULT '',
  local_url  TEXT NOT NULL DEFAULT '',
  repo_url   TEXT NOT NULL DEFAULT '',
  image      TEXT NOT NULL DEFAULT '',
  container  TEXT NOT NULL DEFAULT '',
  error      TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO deployments (id, repository, branch, status, url, created_at)
VALUES ('dep_seed_1', 'octocat/Hello-World', 'main', 'running', 'http://localhost:20001', NOW())
ON CONFLICT (id) DO NOTHING;
