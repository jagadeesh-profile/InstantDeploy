package runtime

import (
	"fmt"
	"strings"
)

// DockerfileGenerator writes Dockerfiles for detected project types.
type DockerfileGenerator struct {
	logf RuntimeLogger
}

// NewDockerfileGenerator returns a new generator. logf may be nil.
func NewDockerfileGenerator(logf RuntimeLogger) *DockerfileGenerator {
	if logf == nil {
		logf = func(_, _ string) {}
	}
	return &DockerfileGenerator{logf: logf}
}

// Generate writes a Dockerfile into repoDir based on the detected project.
// It returns the Dockerfile path and container port.
// If the project has its own Dockerfile it returns its path unchanged.
func (g *DockerfileGenerator) Generate(repoDir string, cfg *ProjectConfig) (dockerfilePath string, containerPort int, err error) {
	if cfg.Type == "custom" {
		return repoDir + "/Dockerfile", cfg.Port, nil
	}

	content, err := generateDockerfileContent(cfg)
	if err != nil {
		return "", 0, err
	}

	path := repoDir + "/Dockerfile.instantdeploy"
	if writeErr := writeFile(path, content); writeErr != nil {
		return "", 0, writeErr
	}

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	g.logf("info", fmt.Sprintf("Generated Dockerfile for %s (port %d)", cfg.Type, port))
	return path, port, nil
}

// generateDockerfileContent builds Dockerfile content from a ProjectConfig.
func generateDockerfileContent(cfg *ProjectConfig) (string, error) {
	switch cfg.Type {
	case "java-spring-boot-gradle":
		return generateSpringBootGradleDockerfile(cfg), nil
	case "java-gradle":
		return generateGradleDockerfile(cfg), nil
	case "java-spring-boot-maven":
		return generateSpringBootMavenDockerfile(cfg), nil
	case "java-maven":
		return generateMavenDockerfile(cfg), nil
	case "node", "node-nextjs", "node-nuxt", "node-vite", "node-cra", "node-express", "node-nestjs":
		return generateNodeDockerfile(cfg), nil
	case "python", "python-django", "python-flask", "python-fastapi", "python-streamlit":
		return generatePythonDockerfile(cfg), nil
	case "go":
		return generateGoDockerfile(cfg), nil
	case "rust":
		return generateRustDockerfile(cfg), nil
	case "php", "php-laravel", "php-symfony":
		return generatePHPDockerfile(cfg), nil
	case "ruby", "ruby-rails":
		return generateRubyDockerfile(cfg), nil
	case "dotnet":
		return generateDotNetDockerfile(cfg), nil
	case "static":
		return generateStaticDockerfile(), nil
	default:
		return generateStaticDockerfile(), nil
	}
}

// ==================== JAVA ====================

func generateSpringBootGradleDockerfile(cfg *ProjectConfig) string {
	java := getOrDefault(cfg.Version, "java", "17")
	gradle := getOrDefault(cfg.Version, "gradle", "8.5")
	return fmt.Sprintf(`FROM gradle:%s-jdk%s AS build
WORKDIR /app
COPY build.gradle* settings.gradle* gradlew* gradle.properties* ./
COPY gradle ./gradle 2>/dev/null || true
RUN if [ -f build.gradle ]; then \
        sed -i '/com\.palantir\.docker/d' build.gradle && \
        sed -i '/com\.bmuschko\.docker/d' build.gradle && \
        sed -i '/gradle-docker/d' build.gradle && \
        sed -i '/com\.google\.cloud\.tools\.jib/d' build.gradle; \
    fi
RUN gradle dependencies --no-daemon --refresh-dependencies || true
COPY src ./src
RUN (gradle clean bootJar --no-daemon -x test -x dockerBuild -x docker || \
     gradle clean build --no-daemon -x test || \
     gradle bootJar --no-daemon -x test || \
     gradle build --no-daemon -x test) && \
    find build/libs -name "*.jar" ! -name "*-plain.jar" ! -name "*-sources.jar" -exec cp {} app.jar \;

FROM eclipse-temurin:%s-jre-alpine
WORKDIR /app
COPY --from=build /app/app.jar app.jar
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s \
    CMD wget -q --tries=1 --spider http://localhost:%d/actuator/health || \
        wget -q --tries=1 --spider http://localhost:%d/ || exit 1
ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0"
ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]
`, gradle, java, java, cfg.Port, cfg.Port, cfg.Port)
}

func generateGradleDockerfile(cfg *ProjectConfig) string {
	java := getOrDefault(cfg.Version, "java", "17")
	gradle := getOrDefault(cfg.Version, "gradle", "8.5")
	return fmt.Sprintf(`FROM gradle:%s-jdk%s AS build
WORKDIR /app
COPY build.gradle* settings.gradle* gradlew* ./
COPY gradle ./gradle 2>/dev/null || true
RUN gradle dependencies --no-daemon || true
COPY . .
RUN (gradle clean build --no-daemon -x test || \
     gradle clean jar --no-daemon -x test || \
     gradle assemble --no-daemon) && \
    find build/libs -name "*.jar" -exec cp {} app.jar \;

FROM eclipse-temurin:%s-jre-alpine
WORKDIR /app
COPY --from=build /app/app.jar app.jar
EXPOSE %d
CMD ["java", "-jar", "app.jar"]
`, gradle, java, java, cfg.Port)
}

func generateSpringBootMavenDockerfile(cfg *ProjectConfig) string {
	java := getOrDefault(cfg.Version, "java", "17")
	return fmt.Sprintf(`FROM maven:3.9-eclipse-temurin-%s AS build
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B || true
COPY src ./src
RUN mvn clean package -DskipTests -B && cp target/*.jar app.jar

FROM eclipse-temurin:%s-jre-alpine
WORKDIR /app
COPY --from=build /app/app.jar app.jar
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s \
    CMD wget -q --spider http://localhost:%d/actuator/health || exit 1
ENTRYPOINT ["java", "-jar", "app.jar"]
`, java, java, cfg.Port, cfg.Port)
}

func generateMavenDockerfile(cfg *ProjectConfig) string {
	java := getOrDefault(cfg.Version, "java", "17")
	return fmt.Sprintf(`FROM maven:3.9-eclipse-temurin-%s AS build
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B || true
COPY . .
RUN mvn clean package -DskipTests -B

FROM eclipse-temurin:%s-jre-alpine
WORKDIR /app
COPY --from=build /app/target/*.jar app.jar
EXPOSE %d
CMD ["java", "-jar", "app.jar"]
`, java, java, cfg.Port)
}

// ==================== NODE ====================

func generateNodeDockerfile(cfg *ProjectConfig) string {
	if cfg.Framework == "nextjs" {
		return generateNextJSDockerfile(cfg)
	}

	node := getOrDefault(cfg.Version, "node", "20")
	pkgMgr := cfg.BuildTool
	if pkgMgr == "" {
		pkgMgr = "npm"
	}

	installCmds := map[string]string{
		"npm":  "npm ci --only=production || npm install --production",
		"yarn": "yarn install --frozen-lockfile --production || yarn install --production",
		"pnpm": "pnpm install --frozen-lockfile --prod || pnpm install --prod",
		"bun":  "bun install --production",
	}
	installCmd := installCmds[pkgMgr]
	if installCmd == "" {
		installCmd = "npm install --production"
	}

	buildCmd := ""
	if cfg.BuildCommand != "" {
		buildCmd = fmt.Sprintf("RUN %s run build || true", pkgMgr)
	}

	startCmd := cfg.StartCommand
	if startCmd == "" {
		startCmd = "start"
	}

	return fmt.Sprintf(`FROM node:%s-alpine AS build
WORKDIR /app
COPY package*.json yarn.lock* pnpm-lock.yaml* bun.lockb* ./
RUN %s
COPY . .
%s

FROM node:%s-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app ./
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:%d/ || exit 1
CMD ["%s", "run", "%s"]
`, node, installCmd, buildCmd, node, cfg.Port, cfg.Port, pkgMgr, startCmd)
}

func generateNextJSDockerfile(cfg *ProjectConfig) string {
	node := getOrDefault(cfg.Version, "node", "20")
	pkgMgr := cfg.BuildTool
	if pkgMgr == "" {
		pkgMgr = "npm"
	}
	return fmt.Sprintf(`FROM node:%s-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN %s install --frozen-lockfile || %s install

FROM node:%s-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN %s run build

FROM node:%s-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup --system --gid 1001 nodejs && adduser --system --uid 1001 nextjs
COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]
`, node, pkgMgr, pkgMgr, node, pkgMgr, node)
}

// ==================== PYTHON ====================

func generatePythonDockerfile(cfg *ProjectConfig) string {
	python := getOrDefault(cfg.Version, "python", "3.11")

	startCmd := "python app.py"
	switch cfg.Framework {
	case "django":
		startCmd = fmt.Sprintf("python manage.py runserver 0.0.0.0:%d", cfg.Port)
	case "fastapi":
		startCmd = fmt.Sprintf("uvicorn main:app --host 0.0.0.0 --port %d", cfg.Port)
	case "flask":
		startCmd = fmt.Sprintf("flask run --host=0.0.0.0 --port=%d", cfg.Port)
	case "streamlit":
		startCmd = fmt.Sprintf("streamlit run app.py --server.port=%d --server.address=0.0.0.0", cfg.Port)
	}

	return fmt.Sprintf(`FROM python:%s-slim
WORKDIR /app
COPY requirements.txt* Pipfile* pyproject.toml* poetry.lock* ./
RUN pip install --no-cache-dir --upgrade pip && \
    (pip install --no-cache-dir -r requirements.txt || \
     (pip install pipenv && pipenv install --system --deploy) || \
     (pip install poetry && poetry install --no-dev) || \
     true)
COPY . .
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s \
    CMD curl -f http://localhost:%d/ || exit 1
CMD %s
`, python, cfg.Port, cfg.Port, startCmd)
}

// ==================== GO ====================

func generateGoDockerfile(cfg *ProjectConfig) string {
	goVersion := getOrDefault(cfg.Version, "go", "1.22")
	return fmt.Sprintf(`FROM golang:%s-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:%d/ || exit 1
CMD ["./main"]
`, goVersion, cfg.Port, cfg.Port)
}

// ==================== RUST ====================

func generateRustDockerfile(cfg *ProjectConfig) string {
	return fmt.Sprintf(`FROM rust:1.75-alpine AS builder
WORKDIR /app
RUN apk add --no-cache musl-dev
COPY Cargo.* ./
RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release || true
COPY . .
RUN cargo build --release

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/target/release/* .
EXPOSE %d
CMD ["./main"]
`, cfg.Port)
}

// ==================== PHP ====================

func generatePHPDockerfile(cfg *ProjectConfig) string {
	php := getOrDefault(cfg.Version, "php", "8.2")
	if cfg.Framework == "laravel" {
		return fmt.Sprintf(`FROM php:%s-fpm-alpine AS build
WORKDIR /app
RUN apk add --no-cache zip unzip git curl
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer
COPY composer.json composer.lock* ./
RUN composer install --no-dev --no-scripts --no-autoloader
COPY . .
RUN composer dump-autoload --optimize

FROM php:%s-fpm-alpine
WORKDIR /app
COPY --from=build /app /app
RUN chown -R www-data:www-data /app/storage /app/bootstrap/cache
EXPOSE 8000
CMD php artisan serve --host=0.0.0.0 --port=8000
`, php, php)
	}
	return fmt.Sprintf(`FROM php:%s-apache
WORKDIR /var/www/html
COPY . .
RUN chown -R www-data:www-data /var/www/html
EXPOSE 80
`, php)
}

// ==================== RUBY ====================

func generateRubyDockerfile(cfg *ProjectConfig) string {
	ruby := getOrDefault(cfg.Version, "ruby", "3.2")
	if cfg.Framework == "rails" {
		return fmt.Sprintf(`FROM ruby:%s-alpine AS build
WORKDIR /app
RUN apk add --no-cache build-base postgresql-dev nodejs yarn
COPY Gemfile Gemfile.lock ./
RUN bundle install --without development test
COPY . .
RUN bundle exec rails assets:precompile || true

FROM ruby:%s-alpine
WORKDIR /app
RUN apk add --no-cache postgresql-client nodejs
COPY --from=build /usr/local/bundle /usr/local/bundle
COPY --from=build /app /app
EXPOSE 3000
CMD ["bundle", "exec", "rails", "server", "-b", "0.0.0.0"]
`, ruby, ruby)
	}
	return fmt.Sprintf(`FROM ruby:%s-alpine
WORKDIR /app
COPY Gemfile Gemfile.lock ./
RUN bundle install
COPY . .
EXPOSE 4567
CMD ["bundle", "exec", "ruby", "app.rb"]
`, ruby)
}

// ==================== .NET ====================

func generateDotNetDockerfile(cfg *ProjectConfig) string {
	dotnet := getOrDefault(cfg.Version, "dotnet", "8.0")
	projectFile := strings.TrimSuffix(cfg.BuildFile, ".csproj")
	return fmt.Sprintf(`FROM mcr.microsoft.com/dotnet/sdk:%s AS build
WORKDIR /app
COPY *.csproj ./
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o out

FROM mcr.microsoft.com/dotnet/aspnet:%s
WORKDIR /app
COPY --from=build /app/out .
EXPOSE 5000
CMD ["dotnet", "%s.dll"]
`, dotnet, dotnet, projectFile)
}

// ==================== STATIC ====================

func generateStaticDockerfile() string {
	return `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:80/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`
}
