package runtime

import (
	"reflect"
	"testing"
)

func TestInferPortFromContainerLogs(t *testing.T) {
	tests := []struct {
		name string
		logs string
		want int
	}{
		{
			name: "extract from running on url",
			logs: " * Running on http://127.0.0.1:5000",
			want: 5000,
		},
		{
			name: "extract from listening message",
			logs: "Server listening on port 3000",
			want: 3000,
		},
		{
			name: "no port found",
			logs: "application starting",
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inferPortFromContainerLogs(tc.logs)
			if got != tc.want {
				t.Fatalf("unexpected inferred port. want %d got %d", tc.want, got)
			}
		})
	}
}

func TestHasLoopbackBindHint(t *testing.T) {
	if !hasLoopbackBindHint("Running on http://127.0.0.1:8000") {
		t.Fatal("expected loopback bind hint to be detected")
	}
	if hasLoopbackBindHint("Listening on 0.0.0.0:8080") {
		t.Fatal("did not expect loopback hint for 0.0.0.0 bind")
	}
}

func TestIsDirectoryListingResponse(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		contentType string
		body        string
		want        bool
	}{
		{
			name:        "directory listing marker",
			statusCode:  200,
			contentType: "text/html",
			body:        "Directory listing for /\n.git/\nREADME.md",
			want:        true,
		},
		{
			name:        "nginx index of page",
			statusCode:  200,
			contentType: "text/html",
			body:        "<html><title>Index of /</title><h1>Index of /</h1></html>",
			want:        true,
		},
		{
			name:        "normal json health response",
			statusCode:  200,
			contentType: "application/json",
			body:        "{\"status\":\"ok\"}",
			want:        false,
		},
		{
			name:        "raw repo root listing",
			statusCode:  200,
			contentType: "text/plain",
			body:        ".git/\nREADME.md\nrequirements.txt\nsetup.py\nDockerfile\n",
			want:        true,
		},
		{
			name:        "normal landing page html",
			statusCode:  200,
			contentType: "text/html",
			body:        "<html><head><title>My App</title></head><body><h1>Welcome</h1></body></html>",
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isDirectoryListingResponse(tc.statusCode, tc.contentType, tc.body)
			if got != tc.want {
				t.Fatalf("unexpected directory listing detection. want %v got %v", tc.want, got)
			}
		})
	}
}

func TestNormalizeRepositoryInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantURL     string
		wantDisplay string
		wantErr     bool
	}{
		{
			name:        "owner repo shorthand",
			input:       "octocat/Hello-World",
			wantURL:     "https://github.com/octocat/Hello-World.git",
			wantDisplay: "octocat/Hello-World",
		},
		{
			name:        "github url normalized",
			input:       "https://github.com/octocat/Hello-World/tree/main",
			wantURL:     "https://github.com/octocat/Hello-World.git",
			wantDisplay: "octocat/Hello-World",
		},
		{
			name:    "invalid host rejected",
			input:   "https://example.com/octocat/Hello-World",
			wantErr: true,
		},
		{
			name:    "invalid shorthand rejected",
			input:   "octocat",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotDisplay, err := normalizeRepositoryInput(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotURL != tc.wantURL {
				t.Fatalf("unexpected repo url. want %q got %q", tc.wantURL, gotURL)
			}
			if gotDisplay != tc.wantDisplay {
				t.Fatalf("unexpected display repo. want %q got %q", tc.wantDisplay, gotDisplay)
			}
		})
	}
}

func TestDockerBuildArgsDefaults(t *testing.T) {
	t.Setenv("BUILD_PULL", "")
	t.Setenv("BUILD_NO_CACHE", "")
	t.Setenv("BUILD_PLATFORM", "")
	t.Setenv("BUILD_TARGET", "")

	args := dockerBuildArgs("img:latest", "Dockerfile.instantdeploy")
	want := []string{"build", "--pull", "-t", "img:latest", "-f", "Dockerfile.instantdeploy", "."}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected build args\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestDockerBuildArgsWithOptionalFlags(t *testing.T) {
	t.Setenv("BUILD_PULL", "false")
	t.Setenv("BUILD_NO_CACHE", "true")
	t.Setenv("BUILD_PLATFORM", "linux/amd64")
	t.Setenv("BUILD_TARGET", "runner")

	args := dockerBuildArgs("img:latest", "Dockerfile.instantdeploy")
	want := []string{"build", "--no-cache", "--platform", "linux/amd64", "--target", "runner", "-t", "img:latest", "-f", "Dockerfile.instantdeploy", "."}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected build args\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestDockerRunArgsDefaults(t *testing.T) {
	t.Setenv("RUN_MEMORY", "")
	t.Setenv("RUN_CPUS", "")
	t.Setenv("RUN_PIDS_LIMIT", "")
	t.Setenv("RUN_RESTART_POLICY", "")

	args := dockerRunArgs("ctr", "img:latest", "dep_123", 20000, 8080)
	want := []string{"run", "-d", "--name", "ctr", "-p", "20000:8080", "--label", "instantdeploy.managed=true", "--label", "instantdeploy.deployment_id=dep_123", "--restart", "no", "img:latest"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected run args\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestDockerRunArgsWithOptionalLimits(t *testing.T) {
	t.Setenv("RUN_MEMORY", "1g")
	t.Setenv("RUN_CPUS", "1.5")
	t.Setenv("RUN_PIDS_LIMIT", "512")
	t.Setenv("RUN_RESTART_POLICY", "on-failure:3")

	args := dockerRunArgs("ctr", "img:latest", "dep_456", 21000, 3000)
	want := []string{"run", "-d", "--name", "ctr", "-p", "21000:3000", "--label", "instantdeploy.managed=true", "--label", "instantdeploy.deployment_id=dep_456", "--memory", "1g", "--cpus", "1.5", "--pids-limit", "512", "--restart", "on-failure:3", "img:latest"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected run args\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestGetBoolEnv(t *testing.T) {
	t.Setenv("FLAG_TRUE", "yes")
	t.Setenv("FLAG_FALSE", "off")
	t.Setenv("FLAG_INVALID", "maybe")
	t.Setenv("FLAG_EMPTY", "")

	if !getBoolEnv("FLAG_TRUE", false) {
		t.Fatal("expected FLAG_TRUE to parse as true")
	}
	if getBoolEnv("FLAG_FALSE", true) {
		t.Fatal("expected FLAG_FALSE to parse as false")
	}
	if !getBoolEnv("FLAG_INVALID", true) {
		t.Fatal("expected invalid boolean to return fallback true")
	}
	if getBoolEnv("FLAG_EMPTY", false) {
		t.Fatal("expected empty boolean to return fallback false")
	}
}
