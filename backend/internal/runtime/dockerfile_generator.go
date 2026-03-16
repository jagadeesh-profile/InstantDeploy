package runtime



import (

    "fmt"

    "strings"

)



// GenerateDockerfile creates optimized Dockerfile for any project type

func GenerateDockerfile(config *ProjectConfig) (string, error) {

    switch config.Type {

    case "custom":

        return "", nil



    // Java projects

    case "java-spring-boot-gradle":

        return generateSpringBootGradleDockerfile(config), nil

    case "java-gradle":

        return generateGradleDockerfile(config), nil

    case "java-spring-boot-maven":

        return generateSpringBootMavenDockerfile(config), nil

    case "java-maven":

        return generateMavenDockerfile(config), nil



    // Node.js projects

    case "node", "node-nextjs", "node-nuxt", "node-vite", "node-cra", "node-express", "node-nestjs":

        return generateNodeDockerfile(config), nil



    // Python projects

    case "python", "python-django", "python-flask", "python-fastapi", "python-streamlit":

        return generatePythonDockerfile(config), nil



    // Go projects

    case "go":

        return generateGoDockerfile(config), nil



    // Rust projects

    case "rust":

        return generateRustDockerfile(config), nil



    // PHP projects

    case "php", "php-laravel", "php-symfony":

        return generatePHPDockerfile(config), nil



    // Ruby projects

    case "ruby", "ruby-rails":

        return generateRubyDockerfile(config), nil



    // .NET projects

    case "dotnet":

        return generateDotNetDockerfile(config), nil



    // Static

    case "static":

        return generateStaticDockerfile(config), nil



    default:

        return "", fmt.Errorf("unsupported project type: %s", config.Type)

    }

}



// ==================== JAVA DOCKERFILES ====================



func generateSpringBootGradleDockerfile(config *ProjectConfig) string {

    java := getOrDefault(config.Version, "java", "17")

    gradle := getOrDefault(config.Version, "gradle", "8.5")



    return fmt.Sprintf(`FROM gradle:%s-jdk%s AS build

WORKDIR /app



# Copy gradle files

COPY build.gradle* settings.gradle* gradlew* gradle.properties* ./

COPY gradle ./gradle 2>/dev/null || true



# FIX: Remove problematic plugins

RUN if [ -f build.gradle ]; then \

        cp build.gradle build.gradle.backup && \

        sed -i '/com\.palantir\.docker/d' build.gradle && \

        sed -i '/com\.bmuschko\.docker/d' build.gradle && \

        sed -i '/gradle-docker/d' build.gradle && \

        sed -i '/com\.google\.cloud\.tools\.jib/d' build.gradle && \

        echo " Removed docker plugins"; \

    fi



# Download dependencies

RUN gradle dependencies --no-daemon --refresh-dependencies || true



# Copy source

COPY src ./src 2>/dev/null || COPY . .



# Build with fallback strategies

RUN (gradle clean bootJar --no-daemon -x test -x dockerBuild -x docker || \

     gradle clean build --no-daemon -x test -x dockerBuild -x docker || \

     gradle bootJar --no-daemon -x test || \

     gradle build --no-daemon -x test) && \

    find build/libs -name "*.jar" ! -name "*-plain.jar" ! -name "*-sources.jar" -exec cp {} app.jar \;



# Runtime

FROM eclipse-temurin:%s-jre-alpine

WORKDIR /app

COPY --from=build /app/app.jar app.jar



EXPOSE %d



HEALTHCHECK --interval=30s --timeout=3s --start-period=60s \

    CMD wget -q --tries=1 --spider http://localhost:%d/actuator/health || \

        wget -q --tries=1 --spider http://localhost:%d/ || exit 1



ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0"

ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]

`, gradle, java, java, config.Port, config.Port, config.Port)

}



func generateGradleDockerfile(config *ProjectConfig) string {

    java := getOrDefault(config.Version, "java", "17")

    gradle := getOrDefault(config.Version, "gradle", "8.5")



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

`, gradle, java, java, config.Port)

}



func generateSpringBootMavenDockerfile(config *ProjectConfig) string {

    java := getOrDefault(config.Version, "java", "17")



    return fmt.Sprintf(`FROM maven:3.9-eclipse-temurin-%s AS build

WORKDIR /app



COPY pom.xml .

RUN mvn dependency:go-offline -B || true



COPY src ./src



RUN mvn clean package -DskipTests -B && \

    cp target/*.jar app.jar



FROM eclipse-temurin:%s-jre-alpine

WORKDIR /app

COPY --from=build /app/app.jar app.jar



EXPOSE %d



HEALTHCHECK --interval=30s --timeout=3s --start-period=60s \

    CMD wget -q --spider http://localhost:%d/actuator/health || exit 1



ENTRYPOINT ["java", "-jar", "app.jar"]

`, java, java, config.Port, config.Port)

}



func generateMavenDockerfile(config *ProjectConfig) string {

    java := getOrDefault(config.Version, "java", "17")



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

`, java, java, config.Port)

}



// ==================== NODE.JS DOCKERFILES ====================



func generateNodeDockerfile(config *ProjectConfig) string {

    node := getOrDefault(config.Version, "node", "20")

    pkgMgr := config.BuildTool

    if pkgMgr == "" {

        pkgMgr = "npm"

    }



    installCmd := map[string]string{

        "npm":  "npm ci --only=production || npm install --production",

        "yarn": "yarn install --frozen-lockfile --production || yarn install --production",

        "pnpm": "pnpm install --frozen-lockfile --prod || pnpm install --prod",

        "bun":  "bun install --production",

    }[pkgMgr]



    buildCmd := ""

    if config.BuildCommand != "" {

        buildCmd = fmt.Sprintf("RUN %s run build || true", pkgMgr)

    }



    startCmd := config.StartCommand

    if startCmd == "" {

        startCmd = "start"

    }



    // Special handling for frameworks

    if config.Framework == "nextjs" {

        return generateNextJSDockerfile(config)

    }



    return fmt.Sprintf(`FROM node:%s-alpine AS build

WORKDIR /app



# Install dependencies

COPY package*.json yarn.lock* pnpm-lock.yaml* bun.lockb* ./

RUN %s



# Copy source

COPY . .



# Build

%s



# Production

FROM node:%s-alpine

WORKDIR /app



ENV NODE_ENV=production



COPY --from=build /app/node_modules ./node_modules

COPY --from=build /app ./



EXPOSE %d



HEALTHCHECK --interval=30s --timeout=3s \

    CMD wget -q --spider http://localhost:%d/ || exit 1



CMD ["%s", "run", "%s"]

`, node, installCmd, buildCmd, node, config.Port, config.Port, pkgMgr, startCmd)

}



func generateNextJSDockerfile(config *ProjectConfig) string {

    node := getOrDefault(config.Version, "node", "20")

    pkgMgr := config.BuildTool

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



RUN addgroup --system --gid 1001 nodejs && \

    adduser --system --uid 1001 nextjs



COPY --from=build /app/public ./public

COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./

COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static



USER nextjs



EXPOSE 3000



CMD ["node", "server.js"]

`, node, pkgMgr, pkgMgr, node, pkgMgr, node)

}



// ==================== PYTHON DOCKERFILES ====================



func generatePythonDockerfile(config *ProjectConfig) string {

    python := getOrDefault(config.Version, "python", "3.11")



    startCmd := "python app.py"

    if config.Framework == "django" {

        startCmd = "python manage.py runserver 0.0.0.0:8000"

    } else if config.Framework == "fastapi" {

        startCmd = "uvicorn main:app --host 0.0.0.0 --port 8000"

    } else if config.Framework == "flask" {

        startCmd = "flask run --host=0.0.0.0 --port=5000"

    } else if config.Framework == "streamlit" {

        startCmd = "streamlit run app.py --server.port=8501 --server.address=0.0.0.0"

    }



    return fmt.Sprintf(`FROM python:%s-slim

WORKDIR /app



# Install dependencies

COPY requirements.txt* Pipfile* pyproject.toml* poetry.lock* ./



RUN pip install --no-cache-dir --upgrade pip && \

    (pip install --no-cache-dir -r requirements.txt || \

     (pip install pipenv && pipenv install --system --deploy) || \

     (pip install poetry && poetry install --no-dev) || \

     true)



# Copy application

COPY . .



EXPOSE %d



HEALTHCHECK --interval=30s --timeout=3s \

    CMD curl -f http://localhost:%d/ || exit 1



CMD %s

`, python, config.Port, config.Port, startCmd)

}



// ==================== GO DOCKERFILES ====================



func generateGoDockerfile(config *ProjectConfig) string {

    goVersion := getOrDefault(config.Version, "go", "1.21")



    return fmt.Sprintf(`FROM golang:%s-alpine AS builder

WORKDIR /app



# Download dependencies

COPY go.* ./

RUN go mod download



# Build

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .



# Runtime

FROM alpine:latest

RUN apk --no-cache add ca-certificates



WORKDIR /root/

COPY --from=builder /app/main .



EXPOSE %d



HEALTHCHECK --interval=30s --timeout=3s \

    CMD wget -q --spider http://localhost:%d/ || exit 1



CMD ["./main"]

`, goVersion, config.Port, config.Port)

}



// ==================== RUST DOCKERFILES ====================



func generateRustDockerfile(config *ProjectConfig) string {

    return fmt.Sprintf(`FROM rust:1.75-alpine AS builder

WORKDIR /app



RUN apk add --no-cache musl-dev



COPY Cargo.* ./

RUN mkdir src && \

    echo "fn main() {}" > src/main.rs && \

    cargo build --release || true



COPY . .

RUN cargo build --release



FROM alpine:latest

RUN apk --no-cache add ca-certificates



WORKDIR /root/

COPY --from=builder /app/target/release/* .



EXPOSE %d

CMD ["./main"]

`, config.Port)

}



// ==================== PHP DOCKERFILES ====================



func generatePHPDockerfile(config *ProjectConfig) string {

    php := getOrDefault(config.Version, "php", "8.2")



    if config.Framework == "laravel" {

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



// ==================== RUBY DOCKERFILES ====================



func generateRubyDockerfile(config *ProjectConfig) string {

    ruby := getOrDefault(config.Version, "ruby", "3.2")



    if config.Framework == "rails" {

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



// ==================== .NET DOCKERFILES ====================



func generateDotNetDockerfile(config *ProjectConfig) string {

    dotnet := getOrDefault(config.Version, "dotnet", "8.0")

    projectFile := config.BuildFile



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

`, dotnet, dotnet, strings.TrimSuffix(projectFile, ".csproj"))

}



// ==================== STATIC DOCKERFILES ====================



func generateStaticDockerfile(config *ProjectConfig) string {

    return `FROM nginx:alpine



COPY . /usr/share/nginx/html



EXPOSE 80



HEALTHCHECK --interval=30s --timeout=3s \

    CMD wget -q --spider http://localhost:80/ || exit 1



CMD ["nginx", "-g", "daemon off;"]

`

}



// ==================== HELPERS ====================



func getOrDefault(m map[string]string, key, defaultValue string) string {

    if val, ok := m[key]; ok && val != "" {

        return val

    }

    return defaultValue

}
