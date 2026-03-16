package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ProjectKind string

const (
	ProjectKindCustom     ProjectKind = "custom"
	ProjectKindNode       ProjectKind = "node"
	ProjectKindPython     ProjectKind = "python"
	ProjectKindGo         ProjectKind = "go"
	ProjectKindJavaMaven  ProjectKind = "java-maven"
	ProjectKindJavaGradle ProjectKind = "java-gradle"
	ProjectKindRust       ProjectKind = "rust"
	ProjectKindPHP        ProjectKind = "php"
	ProjectKindRuby       ProjectKind = "ruby"
	ProjectKindDotNet     ProjectKind = "dotnet"
	ProjectKindStatic     ProjectKind = "static"
)

type DetectedProject struct {
	Kind               ProjectKind
	Framework          string
	RuntimeVersion     string
	PackageManager     string
	Port               int
	HealthCheckPath    string
	ExistingDockerfile string
	BuildCommand       string
	StartCommand       string
	StaticOutputDir    string
	Entrypoint         string
	DotNetProjectFile  string
	JavaUseWrapper     bool
	Summary            string
}

type SmartDetector struct{}

func NewSmartDetector() *SmartDetector {
	return &SmartDetector{}
}

func (d *SmartDetector) Detect(repoDir string) (*DetectedProject, error) {
	if fileExists(filepath.Join(repoDir, "Dockerfile")) {
		project := &DetectedProject{
			Kind:               ProjectKindCustom,
			Framework:          "custom",
			ExistingDockerfile: filepath.Join(repoDir, "Dockerfile"),
			Port:               3000,
			HealthCheckPath:    "/",
			Summary:            "custom Dockerfile",
		}
		if port := detectExposePortFromDockerfile(project.ExistingDockerfile); port > 0 {
			project.Port = port
		}
		return project, nil
	}

	if project, ok, err := d.detectDotNet(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectJava(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectNode(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectPython(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectGo(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectRust(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectPHP(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok, err := d.detectRuby(repoDir); ok || err != nil {
		return project, err
	}
	if project, ok := d.detectStatic(repoDir); ok {
		return project, nil
	}

	return &DetectedProject{
		Kind:            ProjectKindStatic,
		Framework:       "static",
		Port:            80,
		HealthCheckPath: "/",
		Summary:         "static site fallback",
	}, nil
}

type packageJSON struct {
	Name         string            `json:"name"`
	Main         string            `json:"main"`
	PackageMGR   string            `json:"packageManager"`
	Scripts      map[string]string `json:"scripts"`
	Dependencies map[string]string `json:"dependencies"`
	DevDeps      map[string]string `json:"devDependencies"`
	Engines      struct {
		Node string `json:"node"`
	} `json:"engines"`
}

func (d *SmartDetector) detectNode(repoDir string) (*DetectedProject, bool, error) {
	pkgPath := filepath.Join(repoDir, "package.json")
	if !fileExists(pkgPath) {
		return nil, false, nil
	}

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, true, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, true, fmt.Errorf("failed to parse package.json: %w", err)
	}

	framework := detectNodeFramework(pkg)
	packageManager := detectNodePackageManager(repoDir, pkg.PackageMGR)
	version := firstNonEmpty(strings.TrimSpace(pkg.Engines.Node), readTrimmedFile(filepath.Join(repoDir, ".nvmrc")), "20")
	port := 3000
	healthPath := "/"
	staticOutputDir := ""
	buildCommand := nodeRunScript(packageManager, "build")
	startCommand := nodeStartCommand(packageManager, pkg, framework)

	if framework == "vite" || framework == "react-spa" || framework == "vue-spa" || framework == "angular" || framework == "svelte" {
		port = 80
		staticOutputDir = detectNodeStaticOutputDir(repoDir, framework)
		startCommand = ""
	}
	if framework == "next" || framework == "nuxt" || framework == "express" || framework == "nestjs" || framework == "fastify" {
		healthPath = "/"
	}

	return &DetectedProject{
		Kind:            ProjectKindNode,
		Framework:       framework,
		RuntimeVersion:  sanitizeNodeVersion(version),
		PackageManager:  packageManager,
		Port:            port,
		HealthCheckPath: healthPath,
		BuildCommand:    buildCommand,
		StartCommand:    startCommand,
		StaticOutputDir: staticOutputDir,
		Entrypoint:      strings.TrimSpace(pkg.Main),
		Summary:         fmt.Sprintf("node/%s via %s", framework, packageManager),
	}, true, nil
}

func (d *SmartDetector) detectPython(repoDir string) (*DetectedProject, bool, error) {
	if !fileExists(filepath.Join(repoDir, "requirements.txt")) && !fileExists(filepath.Join(repoDir, "pyproject.toml")) && !fileExists(filepath.Join(repoDir, "Pipfile")) {
		return nil, false, nil
	}

	framework := "python"
	port := 8000
	healthPath := "/"
	entrypoint := firstExisting(repoDir, "main.py", "app.py", "run.py", "wsgi.py", "manage.py")
	version := firstNonEmpty(readTrimmedFile(filepath.Join(repoDir, "runtime.txt")), readTrimmedFile(filepath.Join(repoDir, ".python-version")), detectRequiresPython(filepath.Join(repoDir, "pyproject.toml")), "3.11")

	deps := strings.ToLower(readOptional(filepath.Join(repoDir, "requirements.txt")) + "\n" + readOptional(filepath.Join(repoDir, "pyproject.toml")))
	switch {
	case fileExists(filepath.Join(repoDir, "manage.py")) || strings.Contains(deps, "django"):
		framework = "django"
		start := "python manage.py migrate && python manage.py runserver 0.0.0.0:8000"
		return &DetectedProject{Kind: ProjectKindPython, Framework: framework, RuntimeVersion: sanitizePythonVersion(version), Port: 8000, HealthCheckPath: "/", StartCommand: start, Summary: "python/django"}, true, nil
	case strings.Contains(deps, "fastapi") || strings.Contains(deps, "uvicorn"):
		framework = "fastapi"
		module := strings.TrimSuffix(entrypoint, ".py")
		if module == "" {
			module = "main"
		}
		start := fmt.Sprintf("python -m uvicorn %s:app --host 0.0.0.0 --port 8000", module)
		return &DetectedProject{Kind: ProjectKindPython, Framework: framework, RuntimeVersion: sanitizePythonVersion(version), Port: 8000, HealthCheckPath: "/", StartCommand: start, Summary: "python/fastapi"}, true, nil
	case strings.Contains(deps, "streamlit"):
		framework = "streamlit"
		if entrypoint == "" {
			entrypoint = "app.py"
		}
		port = 8501
		start := fmt.Sprintf("streamlit run %s --server.port 8501 --server.address 0.0.0.0", entrypoint)
		return &DetectedProject{Kind: ProjectKindPython, Framework: framework, RuntimeVersion: sanitizePythonVersion(version), Port: port, HealthCheckPath: "/", StartCommand: start, Summary: "python/streamlit"}, true, nil
	default:
		framework = "flask"
		module := strings.TrimSuffix(entrypoint, ".py")
		if module == "" {
			module = "app"
		}
		start := fmt.Sprintf("python -m flask --app %s:app run --host=0.0.0.0 --port=8000", module)
		if entrypoint == "" {
			start = "python app.py"
		}
		return &DetectedProject{Kind: ProjectKindPython, Framework: framework, RuntimeVersion: sanitizePythonVersion(version), Port: port, HealthCheckPath: healthPath, StartCommand: start, Summary: "python/flask"}, true, nil
	}
}

func (d *SmartDetector) detectGo(repoDir string) (*DetectedProject, bool, error) {
	goModPath := filepath.Join(repoDir, "go.mod")
	if !fileExists(goModPath) {
		return nil, false, nil
	}

	goMod := readOptional(goModPath)
	version := "1.22"
	if match := regexp.MustCompile(`(?m)^go\s+([0-9]+(?:\.[0-9]+)?)`).FindStringSubmatch(goMod); len(match) == 2 {
		version = match[1]
	}
	entrypoint := "server"
	if match := regexp.MustCompile(`(?m)^module\s+(.+)$`).FindStringSubmatch(goMod); len(match) == 2 {
		parts := strings.Split(strings.TrimSpace(match[1]), "/")
		entrypoint = sanitizeName(parts[len(parts)-1])
	}

	return &DetectedProject{
		Kind:            ProjectKindGo,
		Framework:       "go",
		RuntimeVersion:  version,
		Port:            8080,
		HealthCheckPath: "/health",
		Entrypoint:      entrypoint,
		Summary:         "go service",
	}, true, nil
}

func (d *SmartDetector) detectJava(repoDir string) (*DetectedProject, bool, error) {
	if fileExists(filepath.Join(repoDir, "pom.xml")) {
		version := sanitizeJavaVersion(detectJavaVersion(filepath.Join(repoDir, "pom.xml"), filepath.Join(repoDir, "mvnw")))
		return &DetectedProject{
			Kind:            ProjectKindJavaMaven,
			Framework:       detectJavaFramework(repoDir),
			RuntimeVersion:  version,
			Port:            8080,
			HealthCheckPath: "/actuator/health",
			Summary:         "java/maven",
		}, true, nil
	}

	if fileExists(filepath.Join(repoDir, "build.gradle")) || fileExists(filepath.Join(repoDir, "build.gradle.kts")) || fileExists(filepath.Join(repoDir, "gradlew")) {
		version := sanitizeJavaVersion(detectJavaVersion(filepath.Join(repoDir, "build.gradle"), filepath.Join(repoDir, "gradlew")))
		return &DetectedProject{
			Kind:            ProjectKindJavaGradle,
			Framework:       detectJavaFramework(repoDir),
			RuntimeVersion:  version,
			Port:            8080,
			HealthCheckPath: "/actuator/health",
			JavaUseWrapper:  fileExists(filepath.Join(repoDir, "gradlew")),
			Summary:         "java/gradle",
		}, true, nil
	}

	return nil, false, nil
}

func (d *SmartDetector) detectRust(repoDir string) (*DetectedProject, bool, error) {
	cargoPath := filepath.Join(repoDir, "Cargo.toml")
	if !fileExists(cargoPath) {
		return nil, false, nil
	}

	content := readOptional(cargoPath)
	entry := "app"
	if match := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`).FindStringSubmatch(content); len(match) == 2 {
		entry = sanitizeName(match[1])
	}

	return &DetectedProject{
		Kind:            ProjectKindRust,
		Framework:       "rust",
		RuntimeVersion:  "1.78",
		Port:            8080,
		HealthCheckPath: "/health",
		Entrypoint:      entry,
		Summary:         "rust service",
	}, true, nil
}

func (d *SmartDetector) detectPHP(repoDir string) (*DetectedProject, bool, error) {
	if !fileExists(filepath.Join(repoDir, "composer.json")) && !fileExists(filepath.Join(repoDir, "artisan")) && !fileExists(filepath.Join(repoDir, "index.php")) {
		return nil, false, nil
	}

	framework := "php"
	start := "php -S 0.0.0.0:8000 -t public"
	if fileExists(filepath.Join(repoDir, "artisan")) {
		framework = "laravel"
		start = "php artisan serve --host=0.0.0.0 --port=8000"
	} else if fileExists(filepath.Join(repoDir, "public", "index.php")) {
		framework = "php-web"
	} else {
		start = "php -S 0.0.0.0:8000"
	}

	return &DetectedProject{
		Kind:            ProjectKindPHP,
		Framework:       framework,
		RuntimeVersion:  "8.3",
		Port:            8000,
		HealthCheckPath: "/",
		StartCommand:    start,
		Summary:         fmt.Sprintf("php/%s", framework),
	}, true, nil
}

func (d *SmartDetector) detectRuby(repoDir string) (*DetectedProject, bool, error) {
	if !fileExists(filepath.Join(repoDir, "Gemfile")) {
		return nil, false, nil
	}

	framework := "ruby"
	start := "bundle exec rackup --host 0.0.0.0 --port 3000"
	if fileExists(filepath.Join(repoDir, "config", "application.rb")) {
		framework = "rails"
		start = "bundle exec rails server -b 0.0.0.0 -p 3000"
	}

	return &DetectedProject{
		Kind:            ProjectKindRuby,
		Framework:       framework,
		RuntimeVersion:  firstNonEmpty(readTrimmedFile(filepath.Join(repoDir, ".ruby-version")), "3.3"),
		Port:            3000,
		HealthCheckPath: "/",
		StartCommand:    start,
		Summary:         fmt.Sprintf("ruby/%s", framework),
	}, true, nil
}

func (d *SmartDetector) detectDotNet(repoDir string) (*DetectedProject, bool, error) {
	files, err := filepath.Glob(filepath.Join(repoDir, "*.csproj"))
	if err != nil {
		return nil, true, fmt.Errorf("failed to inspect dotnet project: %w", err)
	}
	if len(files) == 0 {
		return nil, false, nil
	}

	project := files[0]
	framework := "dotnet"
	if strings.Contains(readOptional(project), "Microsoft.NET.Sdk.Web") {
		framework = "aspnet"
	}

	version := "8.0"
	globalJSON := readOptional(filepath.Join(repoDir, "global.json"))
	if match := regexp.MustCompile(`"version"\s*:\s*"([0-9]+(?:\.[0-9]+)?)`).FindStringSubmatch(globalJSON); len(match) == 2 {
		version = match[1]
	}

	return &DetectedProject{
		Kind:              ProjectKindDotNet,
		Framework:         framework,
		RuntimeVersion:    version,
		Port:              8080,
		HealthCheckPath:   "/health",
		DotNetProjectFile: filepath.Base(project),
		Summary:           fmt.Sprintf("dotnet/%s", framework),
	}, true, nil
}

func (d *SmartDetector) detectStatic(repoDir string) (*DetectedProject, bool) {
	if fileExists(filepath.Join(repoDir, "index.html")) || fileExists(filepath.Join(repoDir, "public", "index.html")) {
		return &DetectedProject{
			Kind:            ProjectKindStatic,
			Framework:       "static",
			Port:            80,
			HealthCheckPath: "/",
			Summary:         "static site",
		}, true
	}
	return nil, false
}

func detectNodeFramework(pkg packageJSON) string {
	deps := mergePackageMaps(pkg.Dependencies, pkg.DevDeps)
	switch {
	case deps["next"] != "":
		return "next"
	case deps["nuxt"] != "" || deps["nuxt3"] != "":
		return "nuxt"
	case deps["@nestjs/core"] != "":
		return "nestjs"
	case deps["express"] != "":
		return "express"
	case deps["fastify"] != "":
		return "fastify"
	case deps["vite"] != "" && deps["react"] != "":
		return "react-spa"
	case deps["vite"] != "" && deps["vue"] != "":
		return "vue-spa"
	case deps["vite"] != "":
		return "vite"
	case deps["react-scripts"] != "":
		return "react-spa"
	case deps["@angular/core"] != "":
		return "angular"
	case deps["svelte"] != "":
		return "svelte"
	default:
		return "node"
	}
}

func detectNodePackageManager(repoDir, packageManagerField string) string {
	field := strings.TrimSpace(strings.ToLower(packageManagerField))
	switch {
	case strings.HasPrefix(field, "pnpm"):
		return "pnpm"
	case strings.HasPrefix(field, "yarn"):
		return "yarn"
	case strings.HasPrefix(field, "bun"):
		return "bun"
	case strings.HasPrefix(field, "npm"):
		return "npm"
	}

	switch {
	case fileExists(filepath.Join(repoDir, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(repoDir, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(repoDir, "bun.lockb")) || fileExists(filepath.Join(repoDir, "bun.lock")):
		return "bun"
	default:
		return "npm"
	}
}

func detectNodeStaticOutputDir(repoDir, framework string) string {
	switch {
	case fileExists(filepath.Join(repoDir, "dist")):
		return "dist"
	case fileExists(filepath.Join(repoDir, "build")):
		return "build"
	case framework == "next":
		return ".next"
	default:
		return "dist"
	}
}

func nodeRunScript(packageManager, script string) string {
	switch packageManager {
	case "pnpm":
		return fmt.Sprintf("pnpm %s", script)
	case "yarn":
		return fmt.Sprintf("yarn %s", script)
	case "bun":
		return fmt.Sprintf("bun run %s", script)
	default:
		return fmt.Sprintf("npm run %s", script)
	}
}

func nodeStartCommand(packageManager string, pkg packageJSON, framework string) string {
	if _, ok := pkg.Scripts["start"]; ok {
		return nodeRunScript(packageManager, "start")
	}
	if framework == "next" {
		return "npx next start -H 0.0.0.0 -p 3000"
	}
	if framework == "nuxt" {
		return "npx nuxt start --host 0.0.0.0 --port 3000"
	}
	if framework == "nestjs" {
		if _, ok := pkg.Scripts["start:prod"]; ok {
			return nodeRunScript(packageManager, "start:prod")
		}
		return "node dist/main.js"
	}
	if strings.TrimSpace(pkg.Main) != "" {
		return fmt.Sprintf("node %s", pkg.Main)
	}
	if _, ok := pkg.Scripts["dev"]; ok {
		return nodeRunScript(packageManager, "dev") + " -- --host 0.0.0.0 --port 3000"
	}
	return "node server.js"
}

func detectJavaFramework(repoDir string) string {
	content := strings.ToLower(readOptional(filepath.Join(repoDir, "pom.xml")) + readOptional(filepath.Join(repoDir, "build.gradle")) + readOptional(filepath.Join(repoDir, "build.gradle.kts")))
	if strings.Contains(content, "spring-boot") || strings.Contains(content, "springframework.boot") {
		return "spring-boot"
	}
	return "java"
}

func detectJavaVersion(paths ...string) string {
	joined := ""
	for _, path := range paths {
		joined += "\n" + readOptional(path)
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`<java.version>([^<]+)</java.version>`),
		regexp.MustCompile(`sourceCompatibility\s*=\s*['"]?([0-9]+(?:\.[0-9]+)?)`),
		regexp.MustCompile(`targetCompatibility\s*=\s*['"]?([0-9]+(?:\.[0-9]+)?)`),
		regexp.MustCompile(`JavaLanguageVersion\.of\(([0-9]+)\)`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(joined); len(match) == 2 {
			return match[1]
		}
	}
	return "17"
}

func detectRequiresPython(pyprojectPath string) string {
	content := readOptional(pyprojectPath)
	if match := regexp.MustCompile(`requires-python\s*=\s*"([^"]+)"`).FindStringSubmatch(content); len(match) == 2 {
		return match[1]
	}
	return ""
}

func sanitizeNodeVersion(version string) string {
	if match := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`).FindStringSubmatch(version); len(match) == 2 {
		return match[1]
	}
	return "20"
}

func sanitizePythonVersion(version string) string {
	if match := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`).FindStringSubmatch(version); len(match) == 2 {
		return match[1]
	}
	return "3.11"
}

func sanitizeJavaVersion(version string) string {
	if match := regexp.MustCompile(`([0-9]+)`).FindStringSubmatch(version); len(match) == 2 {
		switch match[1] {
		case "8", "11", "17", "21":
			return match[1]
		default:
			return "17"
		}
	}
	return "17"
}

func mergePackageMaps(maps ...map[string]string) map[string]string {
	out := make(map[string]string)
	for _, current := range maps {
		for key, value := range current {
			out[key] = value
		}
	}
	return out
}

func readOptional(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func readTrimmedFile(path string) string {
	return strings.TrimSpace(readOptional(path))
}

func firstExisting(repoDir string, names ...string) string {
	for _, name := range names {
		if fileExists(filepath.Join(repoDir, name)) {
			return name
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
