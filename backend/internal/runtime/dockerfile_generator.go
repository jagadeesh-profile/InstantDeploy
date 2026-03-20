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
	// Static frontend projects — build + nginx
	case "node-cra":
		return generateNodeStaticDockerfile(cfg, "build"), nil
	case "node-vite":
		return generateNodeStaticDockerfile(cfg, "dist"), nil
	case "node-static":
		return generateNodeStaticDockerfile(cfg, ""), nil // auto-detect output dir
	// Server-side Node.js projects — keep Node runtime
	// Safety: if type is "node" but there's NO start script, redirect to static builder
	case "node", "node-express", "node-nestjs":
		if cfg.StartCommand == "" && cfg.BuildCommand != "" {
			// No start script but has build → build and serve with nginx
			return generateNodeStaticDockerfile(cfg, ""), nil
		}
		if cfg.StartCommand == "" && cfg.BuildCommand == "" {
			// No start AND no build → serve whatever files exist with nginx
			return generateStaticDockerfile(cfg), nil
		}
		return generateNodeServerDockerfile(cfg), nil
	// Next.js / Nuxt — dedicated templates with their own runtimes
	case "node-nextjs":
		return generateNextJSDockerfile(cfg), nil
	case "node-nuxt":
		return generateNodeServerDockerfile(cfg), nil
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
		return generateStaticDockerfile(cfg), nil
	default:
		return generateStaticDockerfile(cfg), nil
	}
}

// ==================== JAVA ====================

func generateSpringBootGradleDockerfile(cfg *ProjectConfig) string {
	java := getOrDefault(cfg.Version, "java", "17")
	gradle := getOrDefault(cfg.Version, "gradle", "8.5")
	return fmt.Sprintf(`FROM gradle:%s-jdk%s AS build
WORKDIR /app
COPY . .
RUN if [ -f build.gradle ]; then \
        sed -i '/com\.palantir\.docker/d' build.gradle && \
        sed -i '/com\.bmuschko\.docker/d' build.gradle && \
        sed -i '/gradle-docker/d' build.gradle && \
        sed -i '/com\.google\.cloud\.tools\.jib/d' build.gradle; \
    fi
RUN gradle dependencies --no-daemon --refresh-dependencies || true
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
COPY . .
RUN gradle dependencies --no-daemon || true
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

// ==================== NODE: STATIC FRONTEND (CRA / Vite / React / Vue) ====================

// generateNodeStaticDockerfile creates a multi-stage Dockerfile:
//
//	Stage 1: Install ALL deps (including devDependencies) and run build
//	Stage 2: Copy built output to nginx:alpine, serve on port 8080
//
// outputDir hint: "build" for CRA, "dist" for Vite, "" for auto-detect.
func generateNodeStaticDockerfile(cfg *ProjectConfig, outputDir string) string {
	node := getOrDefault(cfg.Version, "node", "20")
	pkgMgr := cfg.BuildTool
	if pkgMgr == "" {
		pkgMgr = "npm"
	}
	bootstrap := nodePackageManagerBootstrap(pkgMgr)

	// Install ALL deps (devDependencies are needed for build tools like vite, react-scripts)
	installCmds := map[string]string{
		"npm":  "npm ci || npm install",
		"yarn": "yarn install --frozen-lockfile || yarn install",
		"pnpm": "pnpm install --frozen-lockfile || pnpm install",
		"bun":  "bun install",
	}
	installCmd := installCmds[pkgMgr]
	if installCmd == "" {
		installCmd = "npm install"
	}

	buildCmd := fmt.Sprintf("%s run build", pkgMgr)

	return fmt.Sprintf(`# Stage 1: Build the application
FROM node:%s-alpine AS build
WORKDIR /app
%s
COPY package*.json yarn.lock* pnpm-lock.yaml* bun.lockb* ./
RUN %s
COPY . .
RUN %s

# Debug: show what the build produced (visible in deployment logs)
RUN echo "=== Build output directories ===" && \
    for dir in dist build out .next public _site; do \
        if [ -d "$dir" ]; then echo "FOUND: /app/$dir ($(ls -1 $dir | wc -l) files)"; fi; \
    done && \
    echo "=== Looking for index.html ===" && \
    find . -name "index.html" -maxdepth 3 -not -path "./node_modules/*" 2>/dev/null || true

# Write nginx config for port 8080 with SPA routing
RUN echo 'server {'                                          > /tmp/nginx.conf && \
    echo '    listen 8080;'                                 >> /tmp/nginx.conf && \
    echo '    server_name _;'                               >> /tmp/nginx.conf && \
    echo '    root /usr/share/nginx/html;'                  >> /tmp/nginx.conf && \
    echo '    index index.html index.htm;'                  >> /tmp/nginx.conf && \
    echo '    location / {'                                 >> /tmp/nginx.conf && \
    echo '        try_files $uri $uri/ /index.html;'        >> /tmp/nginx.conf && \
    echo '    }'                                            >> /tmp/nginx.conf && \
    echo '}'                                               >> /tmp/nginx.conf

# Stage 2: Serve with nginx on port 8080
FROM nginx:alpine
# Remove ALL default nginx content and config
RUN rm -rf /usr/share/nginx/html/* /etc/nginx/conf.d/default.conf
# Install our custom config
COPY --from=build /tmp/nginx.conf /etc/nginx/conf.d/default.conf
# Copy the entire build output — we'll sort out the right files next
COPY --from=build /app/ /tmp/buildout/
# Smart copy: find the built output directory and copy it to nginx html root
RUN set -e; \
    FOUND=false; \
    for dir in %s dist build out .next/out _site public; do \
        if [ -d "/tmp/buildout/$dir" ] && [ "$(ls -A /tmp/buildout/$dir 2>/dev/null)" ]; then \
            cp -r /tmp/buildout/$dir/* /usr/share/nginx/html/ 2>/dev/null && FOUND=true && break; \
        fi; \
    done; \
    if [ "$FOUND" = "false" ]; then \
        IDX=$(find /tmp/buildout -name "index.html" -maxdepth 3 -not -path "*/node_modules/*" -print -quit 2>/dev/null); \
        if [ -n "$IDX" ]; then \
            SRCDIR=$(dirname "$IDX"); \
            cp -r "$SRCDIR"/* /usr/share/nginx/html/; \
            FOUND=true; \
        fi; \
    fi; \
    if [ "$FOUND" = "false" ]; then \
        echo "WARNING: No built output found, copying all non-node_modules files"; \
        find /tmp/buildout -maxdepth 1 -not -name node_modules -not -name ".git" -not -path /tmp/buildout -exec cp -r {} /usr/share/nginx/html/ \; ; \
    fi; \
    rm -rf /tmp/buildout; \
    echo "=== Final nginx html contents ===" && ls -la /usr/share/nginx/html/
# Ensure index.html is at the root — if it ended up in a subdirectory, relocate it
RUN if [ ! -f /usr/share/nginx/html/index.html ]; then \
        echo "index.html not at root, searching..."; \
        IDX=$(find /usr/share/nginx/html -name "index.html" -print -quit 2>/dev/null); \
        if [ -n "$IDX" ]; then \
            SRCDIR=$(dirname "$IDX"); \
            echo "Found at: $IDX — copying contents of $SRCDIR to root"; \
            cp -r "$SRCDIR"/* /usr/share/nginx/html/ 2>/dev/null || true; \
        else \
            echo "WARNING: No index.html found anywhere"; \
        fi; \
    fi
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:8080/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`, node, bootstrap, installCmd, buildCmd, quoteOutputDir(outputDir))
}

// quoteOutputDir returns the outputDir hint for the shell priority list,
// or an empty string placeholder that won't match any directory.
func quoteOutputDir(dir string) string {
	if dir == "" {
		return "_no_hint_"
	}
	return dir
}


// ==================== NODE: SERVER APPS (Express / NestJS / generic) ====================

func generateNodeServerDockerfile(cfg *ProjectConfig) string {
	node := getOrDefault(cfg.Version, "node", "20")
	pkgMgr := cfg.BuildTool
	if pkgMgr == "" {
		pkgMgr = "npm"
	}
	bootstrap := nodePackageManagerBootstrap(pkgMgr)

	// Install ALL deps first (build step may need devDeps), then prune for production
	installCmd := map[string]string{
		"npm":  "npm ci || npm install",
		"yarn": "yarn install --frozen-lockfile || yarn install",
		"pnpm": "pnpm install --frozen-lockfile || pnpm install",
		"bun":  "bun install",
	}[pkgMgr]
	if installCmd == "" {
		installCmd = "npm install"
	}

	buildStep := ""
	if cfg.BuildCommand != "" {
		buildStep = fmt.Sprintf("RUN %s run build || true", pkgMgr)
	}

	startCmd := "start"
	rawStartCmd := ""
	if cfg.StartCommand != "" {
		if strings.Contains(cfg.StartCommand, " ") {
			rawStartCmd = cfg.StartCommand
		} else {
			startCmd = cfg.StartCommand
		}
	}

	port := cfg.Port
	if port == 0 {
		port = 3000
	}

 	cmdLine := fmt.Sprintf(`CMD ["%s", "run", "%s"]`, pkgMgr, startCmd)
	if rawStartCmd != "" {
		cmdLine = fmt.Sprintf(`CMD ["sh", "-c", %q]`, rawStartCmd)
	} else if cfg.StartCommand == "" {
		cmdLine = fmt.Sprintf(`CMD ["sh", "-c", "if [ -f server.js ]; then node server.js; elif [ -f index.js ]; then node index.js; elif [ -f main.js ]; then node main.js; else %s run %s; fi"]`, pkgMgr, startCmd)
	}

	return fmt.Sprintf(`FROM node:%s-alpine AS build
WORKDIR /app
%s
COPY package*.json yarn.lock* pnpm-lock.yaml* bun.lockb* ./
RUN %s
COPY . .
%s

FROM node:%s-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=%d
ENV HOST=0.0.0.0
%s
COPY --from=build /app ./
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:%d/ || exit 1
%s
`, node, bootstrap, installCmd, buildStep, node, port, bootstrap, port, port, cmdLine)
}

// ==================== NODE: NEXT.JS ====================

func generateNextJSDockerfile(cfg *ProjectConfig) string {
	node := getOrDefault(cfg.Version, "node", "20")
	pkgMgr := cfg.BuildTool
	if pkgMgr == "" {
		pkgMgr = "npm"
	}
	bootstrap := nodePackageManagerBootstrap(pkgMgr)
	return fmt.Sprintf(`FROM node:%s-alpine AS deps
WORKDIR /app
%s
COPY package*.json ./
RUN %s install --frozen-lockfile || %s install

FROM node:%s-alpine AS build
WORKDIR /app
%s
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
`, node, bootstrap, pkgMgr, pkgMgr, node, bootstrap, pkgMgr, node)
}

func nodePackageManagerBootstrap(pkgMgr string) string {
	switch pkgMgr {
	case "pnpm":
		return "RUN npm install -g pnpm"
	case "yarn":
		return "RUN npm install -g yarn"
	case "bun":
		return "RUN npm install -g bun"
	default:
		return ""
	}
}

// ==================== PYTHON ====================

func generatePythonDockerfile(cfg *ProjectConfig) string {
	python := getOrDefault(cfg.Version, "python", "3.11")

	startCmd := `if [ -f app.py ]; then python app.py; elif [ -f main.py ]; then python main.py; elif [ -f server.py ]; then python server.py; elif [ -f run.py ]; then python run.py; else python -m http.server ` + fmt.Sprintf("%d", cfg.Port) + ` --bind 0.0.0.0; fi`
	switch cfg.Framework {
	case "django":
		startCmd = fmt.Sprintf("if [ -f manage.py ]; then python manage.py runserver 0.0.0.0:%d; else python -m http.server %d --bind 0.0.0.0; fi", cfg.Port, cfg.Port)
	case "fastapi":
		startCmd = fmt.Sprintf("if [ -f main.py ]; then uvicorn main:app --host 0.0.0.0 --port %d; elif [ -f app.py ]; then uvicorn app:app --host 0.0.0.0 --port %d; else python -m http.server %d --bind 0.0.0.0; fi", cfg.Port, cfg.Port, cfg.Port)
	case "flask":
		startCmd = fmt.Sprintf("if [ -f app.py ]; then export FLASK_APP=app.py; elif [ -f main.py ]; then export FLASK_APP=main.py; fi; flask run --host=0.0.0.0 --port=%d", cfg.Port)
	case "streamlit":
		startCmd = fmt.Sprintf("if [ -f app.py ]; then streamlit run app.py --server.port=%d --server.address=0.0.0.0; elif [ -f main.py ]; then streamlit run main.py --server.port=%d --server.address=0.0.0.0; elif [ -f streamlit_app.py ]; then streamlit run streamlit_app.py --server.port=%d --server.address=0.0.0.0; else python -m http.server %d --bind 0.0.0.0; fi", cfg.Port, cfg.Port, cfg.Port, cfg.Port)
	default:
		if cfg.StartCommand != "" {
			startCmd = cfg.StartCommand
		}
	}

	return fmt.Sprintf(`FROM python:%s-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*
COPY . .
RUN pip install --no-cache-dir --upgrade pip && \
	( [ -f requirements.txt ] && pip install --no-cache-dir -r requirements.txt || true ) && \
	( [ -f Pipfile ] && (pip install pipenv && pipenv install --system --deploy) || true ) && \
	( [ -f pyproject.toml ] && (pip install poetry && poetry install --no-dev) || true )
EXPOSE %d
HEALTHCHECK --interval=30s --timeout=3s \
    CMD curl -f http://localhost:%d/ || exit 1
CMD ["sh", "-c", %q]
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
RUN CGO_ENABLED=0 GOOS=linux (go build -a -installsuffix cgo -o main . || go build -a -installsuffix cgo -o main ./cmd/server || go build -a -installsuffix cgo -o main ./cmd/...)

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

func generateStaticDockerfile(cfg *ProjectConfig) string {
	// Determine what to copy to nginx's html root
	copyCmd := "COPY . /usr/share/nginx/html"
	if cfg.OutputDir != "" && cfg.OutputDir != "." {
		// index.html is inside a subdirectory — copy only that subdirectory's contents
		copyCmd = fmt.Sprintf("COPY %s/ /usr/share/nginx/html/", cfg.OutputDir)
	}

	return fmt.Sprintf(`FROM nginx:alpine
RUN rm -rf /usr/share/nginx/html/* /etc/nginx/conf.d/default.conf
RUN echo 'server {'                                          > /etc/nginx/conf.d/default.conf && \
    echo '    listen 8080;'                                 >> /etc/nginx/conf.d/default.conf && \
    echo '    server_name _;'                               >> /etc/nginx/conf.d/default.conf && \
    echo '    root /usr/share/nginx/html;'                  >> /etc/nginx/conf.d/default.conf && \
    echo '    index index.html index.htm;'                  >> /etc/nginx/conf.d/default.conf && \
    echo '    location / {'                                 >> /etc/nginx/conf.d/default.conf && \
    echo '        try_files $uri $uri/ /index.html;'        >> /etc/nginx/conf.d/default.conf && \
    echo '    }'                                            >> /etc/nginx/conf.d/default.conf && \
    echo '}'                                               >> /etc/nginx/conf.d/default.conf
%s
# Ensure index.html is at the root — if it ended up in a subdirectory, relocate it
RUN if [ ! -f /usr/share/nginx/html/index.html ]; then \
        echo "index.html not at root, searching..."; \
        IDX=$(find /usr/share/nginx/html -name "index.html" -print -quit 2>/dev/null); \
        if [ -n "$IDX" ]; then \
            SRCDIR=$(dirname "$IDX"); \
            echo "Found at: $IDX — copying contents of $SRCDIR to root"; \
            cp -r "$SRCDIR"/* /usr/share/nginx/html/ 2>/dev/null || true; \
        else \
            echo "WARNING: No index.html found anywhere"; \
        fi; \
    fi && \
    echo "=== nginx html root ==="  && ls -la /usr/share/nginx/html/
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:8080/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`, copyCmd)
}

