CREATE TABLE IF NOT EXISTS deployments (
	id TEXT PRIMARY KEY,
	repository TEXT NOT NULL,
	branch TEXT NOT NULL,
	status TEXT NOT NULL,
	url TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO deployments (id, repository, branch, status, url)
VALUES
	('dep_seed_1', 'octocat/Hello-World', 'main', 'running', 'https://dep_seed_1.instantdeploy.app')
ON CONFLICT (id) DO NOTHING;

