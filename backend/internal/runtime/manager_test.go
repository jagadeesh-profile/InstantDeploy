package runtime

import (
	"reflect"
	"testing"
)

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

	args := dockerRunArgs("ctr", "img:latest", 20000, 8080)
	want := []string{"run", "-d", "--name", "ctr", "-p", "20000:8080", "--restart", "unless-stopped", "img:latest"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected run args\nwant: %#v\ngot:  %#v", want, args)
	}
}

func TestDockerRunArgsWithOptionalLimits(t *testing.T) {
	t.Setenv("RUN_MEMORY", "1g")
	t.Setenv("RUN_CPUS", "1.5")
	t.Setenv("RUN_PIDS_LIMIT", "512")
	t.Setenv("RUN_RESTART_POLICY", "on-failure:3")

	args := dockerRunArgs("ctr", "img:latest", 21000, 3000)
	want := []string{"run", "-d", "--name", "ctr", "-p", "21000:3000", "--memory", "1g", "--cpus", "1.5", "--pids-limit", "512", "--restart", "on-failure:3", "img:latest"}
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
