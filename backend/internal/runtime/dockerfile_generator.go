package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DockerfileGenerator struct {
	logf RuntimeLogger
}

func NewDockerfileGenerator(logf RuntimeLogger) *DockerfileGenerator {
	return &DockerfileGenerator{logf: logf}
}

func (g *DockerfileGenerator) Generate(repoDir string, project *DetectedProject) (string, int, error) {
	if project == nil {
		return "", 0, fmt.Errorf("project detection returned nil")
	}
	if project.ExistingDockerfile != "" {
		port := project.Port
		if port == 0 {
			port = detectExposePortFromDockerfile(project.ExistingDockerfile)
		}
		if port == 0 {
			port = 3000
		}
		return project.ExistingDockerfile, port, nil
	}

	var content string
	switch project.Kind {
	case ProjectKindNode:
		content = g.generateNodeDockerfile(project)
	case ProjectKindPython:
		content = g.generatePythonDockerfile(repoDir, project)
	case ProjectKindGo:
		content = g.generateGoDockerfile(project)
	case ProjectKindJavaMaven:
		content = g.generateMavenDockerfile(project)
	case ProjectKindJavaGradle:
		content = g.generateGradleDockerfile(project)
	case ProjectKindRust:
		content = g.generateRustDockerfile(project)
	case ProjectKindPHP:
		content = g.generatePHPDockerfile(repoDir, project)
	case ProjectKindRuby:
		content = g.generateRubyDockerfile(project)
	case ProjectKindDotNet:
		content = g.generateDotNetDockerfile(repoDir, project)
	default:
		content = g.generateStaticDockerfile(project)
	}

	path := filepath.Join(repoDir, "Dockerfile.instantdeploy")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", 0, fmt.Errorf("failed to write generated Dockerfile: %w", err)
	}
	g.log("info", fmt.Sprintf("Generated Dockerfile for %s", project.Summary))
	return path, project.Port, nil
}

func (g *DockerfileGenerator) generateNodeDockerfile(project *DetectedProject) string {
	if project.StaticOutputDir != "" {
		return fmt.Sprintf(`FROM %s AS build
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* bun.lockb* bun.lock* ./
%s
COPY . .
RUN %s

FROM nginx:1.27-alpine
COPY --from=build /app/%s /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`, nodeBuilderImage(project.PackageManager), nodeInstallCommand(project.PackageManager), safeNodeBuildCommand(project), project.StaticOutputDir)
	}

	return fmt.Sprintf(`FROM %s AS build
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* bun.lockb* bun.lock* ./
%s
COPY . .
%s

FROM %s
WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=%d
COPY --from=build /app /app
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:%d%s || exit 1
CMD ["sh", "-c", "%s"]
`, nodeBuilderImage(project.PackageManager), nodeInstallCommand(project.PackageManager), optionalRunLine(project.BuildCommand), nodeRuntimeImage(project.PackageManager), project.Port, project.Port, project.Port, healthPath(project), project.StartCommand)
}

func (g *DockerfileGenerator) generatePythonDockerfile(repoDir string, project *DetectedProject) string {
	installCmd := `COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt`
	if fileExists(filepath.Join(repoDir, "pyproject.toml")) || fileExists(filepath.Join(repoDir, "Pipfile")) {
		installCmd = `COPY requirements.txt* pyproject.toml* poetry.lock* Pipfile* Pipfile.lock* ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi; \
    if [ -f pyproject.toml ]; then pip install --no-cache-dir .; fi`
	}
	return fmt.Sprintf(`FROM python:%s-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1
RUN apt-get update && apt-get install -y --no-install-recommends curl build-essential && rm -rf /var/lib/apt/lists/*
%s
COPY . .
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:%d%s || exit 1
CMD ["sh", "-c", "%s"]
`, project.RuntimeVersion, installCmd, project.Port, project.Port, healthPath(project), project.StartCommand)
}

func (g *DockerfileGenerator) generateGoDockerfile(project *DetectedProject) string {
	entry := firstNonEmpty(project.Entrypoint, "server")
	return fmt.Sprintf(`FROM golang:%s-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tmp/%s .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=build /tmp/%s /usr/local/bin/app
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:%d%s || exit 1
CMD ["/usr/local/bin/app"]
`, project.RuntimeVersion, entry, entry, project.Port, project.Port, healthPath(project))
}

func (g *DockerfileGenerator) generateMavenDockerfile(project *DetectedProject) string {
	javaVersion := sanitizeJavaVersion(project.RuntimeVersion)
	return fmt.Sprintf(`FROM maven:3.9-eclipse-temurin-%s AS build
WORKDIR /app
COPY . .
RUN mvn -q -DskipTests package

FROM eclipse-temurin:%s-jre
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/target/*.jar /app/app.jar
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:%d%s || exit 1
CMD ["java", "-jar", "/app/app.jar"]
`, javaVersion, javaVersion, project.Port, project.Port, healthPath(project))
}

func (g *DockerfileGenerator) generateGradleDockerfile(project *DetectedProject) string {
	javaVersion := sanitizeJavaVersion(project.RuntimeVersion)
	gradleCmd := `gradle --no-daemon clean bootJar --init-script /app/.instantdeploy.init.gradle || gradle --no-daemon clean build --init-script /app/.instantdeploy.init.gradle`
	if project.JavaUseWrapper {
		gradleCmd = `chmod +x ./gradlew && ./gradlew --no-daemon clean bootJar --init-script /app/.instantdeploy.init.gradle || ./gradlew --no-daemon clean build --init-script /app/.instantdeploy.init.gradle`
	}
	return fmt.Sprintf(`FROM gradle:8.7-jdk%s AS build
WORKDIR /app
COPY . .
RUN %s

FROM eclipse-temurin:%s-jre
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/build/libs/*.jar /app/app.jar
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:%d%s || exit 1
CMD ["java", "-jar", "/app/app.jar"]
`, javaVersion, gradleCmd, javaVersion, project.Port, project.Port, healthPath(project))
}

func (g *DockerfileGenerator) generateRustDockerfile(project *DetectedProject) string {
	entry := firstNonEmpty(project.Entrypoint, "app")
	return fmt.Sprintf(`FROM rust:%s-slim AS build
WORKDIR /app
COPY . .
RUN cargo build --release

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/target/release/%s /usr/local/bin/app
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:%d%s || exit 1
CMD ["/usr/local/bin/app"]
`, project.RuntimeVersion, entry, project.Port, project.Port, healthPath(project))
}

func (g *DockerfileGenerator) generatePHPDockerfile(repoDir string, project *DetectedProject) string {
	if fileExists(filepath.Join(repoDir, "composer.json")) {
		return fmt.Sprintf(`FROM composer:2 AS composer
WORKDIR /app
COPY composer.json composer.lock* ./
RUN composer install --no-interaction --prefer-dist --no-dev

FROM php:%s-cli
WORKDIR /app
COPY . .
COPY --from=composer /app/vendor /app/vendor
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD php -r 'exit(@file_get_contents("http://127.0.0.1:%d%s") ? 0 : 1);'
CMD ["sh", "-c", "%s"]
`, project.RuntimeVersion, project.Port, project.Port, healthPath(project), project.StartCommand)
	}

	return fmt.Sprintf(`FROM composer:2 AS composer
FROM php:%s-cli
WORKDIR /app
COPY . .
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD php -r 'exit(@file_get_contents("http://127.0.0.1:%d%s") ? 0 : 1);'
CMD ["sh", "-c", "%s"]
`, project.RuntimeVersion, project.Port, project.Port, healthPath(project), project.StartCommand)
}

func (g *DockerfileGenerator) generateRubyDockerfile(project *DetectedProject) string {
	return fmt.Sprintf(`FROM ruby:%s-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends build-essential curl && rm -rf /var/lib/apt/lists/*
COPY Gemfile Gemfile.lock* ./
RUN bundle install
COPY . .
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:%d%s || exit 1
CMD ["sh", "-c", "%s"]
`, project.RuntimeVersion, project.Port, project.Port, healthPath(project), project.StartCommand)
}

func (g *DockerfileGenerator) generateDotNetDockerfile(repoDir string, project *DetectedProject) string {
	projectFile := strings.TrimSpace(project.DotNetProjectFile)
	if projectFile == "" {
		matches, _ := filepath.Glob(filepath.Join(repoDir, "*.csproj"))
		if len(matches) > 0 {
			projectFile = filepath.Base(matches[0])
		}
	}
	dll := strings.TrimSuffix(filepath.Base(projectFile), filepath.Ext(projectFile)) + ".dll"
	return fmt.Sprintf(`FROM mcr.microsoft.com/dotnet/sdk:%s AS build
WORKDIR /src
COPY . .
RUN dotnet publish %q -c Release -o /app/publish

FROM mcr.microsoft.com/dotnet/aspnet:%s
WORKDIR /app
ENV ASPNETCORE_URLS=http://0.0.0.0:%d
COPY --from=build /app/publish .
EXPOSE %d
CMD ["dotnet", %q]
`, project.RuntimeVersion, projectFile, project.RuntimeVersion, project.Port, project.Port, dll)
}

func (g *DockerfileGenerator) generateStaticDockerfile(project *DetectedProject) string {
	return `FROM nginx:1.27-alpine
COPY . /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`
}

func nodeBuilderImage(packageManager string) string {
	if packageManager == "bun" {
		return "oven/bun:1.1"
	}
	return "node:20-alpine"
}

func nodeRuntimeImage(packageManager string) string {
	if packageManager == "bun" {
		return "oven/bun:1.1"
	}
	return "node:20-alpine"
}

func nodeInstallCommand(packageManager string) string {
	switch packageManager {
	case "pnpm":
		return "RUN corepack enable && (pnpm install --frozen-lockfile || pnpm install)"
	case "yarn":
		return "RUN corepack enable && (yarn install --frozen-lockfile || yarn install)"
	case "bun":
		return "RUN bun install --frozen-lockfile || bun install"
	default:
		return "RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi"
	}
}

func safeNodeBuildCommand(project *DetectedProject) string {
	if strings.TrimSpace(project.BuildCommand) == "" {
		return "echo 'No build script detected'"
	}
	return project.BuildCommand
}

func optionalRunLine(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	return "RUN " + command
}

func healthPath(project *DetectedProject) string {
	if project == nil || strings.TrimSpace(project.HealthCheckPath) == "" {
		return "/"
	}
	return project.HealthCheckPath
}

func (g *DockerfileGenerator) log(level, message string) {
	if g.logf != nil {
		g.logf(level, message)
	}
}
