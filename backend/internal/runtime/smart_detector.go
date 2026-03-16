package runtime

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ProjectConfig holds comprehensive project configuration detected from a repo.
type ProjectConfig struct {
	Type           string            `json:"type"`
	Language       string            `json:"language"`
	Framework      string            `json:"framework"`
	BuildTool      string            `json:"build_tool"`
	Version        map[string]string `json:"version"`
	BuildFile      string            `json:"build_file"`
	MainClass      string            `json:"main_class"`
	Port           int               `json:"port"`
	BuildCommand   string            `json:"build_command"`
	StartCommand   string            `json:"start_command"`
	EnvVars        map[string]string `json:"env_vars"`
	SkipPlugins    []string          `json:"skip_plugins"`
	FixRequired    bool              `json:"fix_required"`
	Summary        string            `json:"summary"`
	CustomSettings map[string]any    `json:"custom_settings"`
}

// SmartDetector intelligently detects and analyses projects.
type SmartDetector struct{}

// NewSmartDetector returns a new SmartDetector.
func NewSmartDetector() *SmartDetector {
	return &SmartDetector{}
}

// Detect performs comprehensive project detection for the given repo directory.
func (d *SmartDetector) Detect(repoDir string) (*ProjectConfig, error) {
	cfg := &ProjectConfig{
		Version:        make(map[string]string),
		EnvVars:        make(map[string]string),
		CustomSettings: make(map[string]any),
		SkipPlugins:    []string{},
		Port:           8080,
		FixRequired:    false,
	}

	h := &detectorHelper{repoDir: repoDir}

	// Priority: custom Dockerfile > Java > Node > Python > Go > Rust > PHP > Ruby > .NET > static
	if h.hasFile("Dockerfile") {
		cfg.Type = "custom"
		cfg.Language = "docker"
		cfg.Summary = "custom Dockerfile"
		return cfg, nil
	}
	if ok, err := h.detectJava(cfg); ok {
		cfg.Summary = fmt.Sprintf("%s (%s)", cfg.Type, cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectNode(cfg); ok {
		cfg.Summary = fmt.Sprintf("%s (%s)", cfg.Type, cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectPython(cfg); ok {
		cfg.Summary = fmt.Sprintf("%s (%s)", cfg.Type, cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectGo(cfg); ok {
		cfg.Summary = fmt.Sprintf("go (%s)", cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectRust(cfg); ok {
		cfg.Summary = fmt.Sprintf("rust (%s)", cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectPHP(cfg); ok {
		cfg.Summary = fmt.Sprintf("%s (%s)", cfg.Type, cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectRuby(cfg); ok {
		cfg.Summary = fmt.Sprintf("%s (%s)", cfg.Type, cfg.Framework)
		return cfg, err
	}
	if ok, err := h.detectDotNet(cfg); ok {
		cfg.Summary = "dotnet"
		return cfg, err
	}

	// Default to static site
	cfg.Type = "static"
	cfg.Language = "html"
	cfg.Port = 80
	cfg.Summary = "static HTML"
	return cfg, nil
}

// ---- internal helper keeps repoDir in one place ----

type detectorHelper struct {
	repoDir string
}

func (h *detectorHelper) hasFile(name string) bool {
	_, err := os.Stat(filepath.Join(h.repoDir, name))
	return err == nil
}

func (h *detectorHelper) readFile(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(h.repoDir, name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ==================== JAVA ====================

func (h *detectorHelper) detectJava(cfg *ProjectConfig) (bool, error) {
	if h.hasFile("build.gradle") || h.hasFile("build.gradle.kts") {
		return h.detectGradle(cfg)
	}
	if h.hasFile("pom.xml") {
		return h.detectMaven(cfg)
	}
	return false, nil
}

func (h *detectorHelper) detectGradle(cfg *ProjectConfig) (bool, error) {
	cfg.Language = "java"
	cfg.BuildTool = "gradle"

	buildFile := "build.gradle"
	if h.hasFile("build.gradle.kts") {
		buildFile = "build.gradle.kts"
	}
	cfg.BuildFile = buildFile

	content, err := h.readFile(buildFile)
	if err != nil {
		return true, err
	}

	if strings.Contains(content, "spring-boot") || strings.Contains(content, "org.springframework.boot") {
		cfg.Framework = "spring-boot"
		cfg.Type = "java-spring-boot-gradle"
	} else {
		cfg.Type = "java-gradle"
	}

	cfg.SkipPlugins = h.detectProblematicGradlePlugins(content)
	if len(cfg.SkipPlugins) > 0 {
		cfg.FixRequired = true
	}

	cfg.Version["java"] = h.extractGradleJavaVersion(content)
	if cfg.Version["java"] == "" {
		cfg.Version["java"] = "17"
	}
	cfg.Version["gradle"] = h.detectGradleWrapperVersion()
	if cfg.Version["gradle"] == "" {
		cfg.Version["gradle"] = "8.5"
	}

	cfg.MainClass = h.detectSpringBootMainClass()
	cfg.Port = h.detectJavaPort()
	return true, nil
}

func (h *detectorHelper) detectMaven(cfg *ProjectConfig) (bool, error) {
	cfg.Language = "java"
	cfg.BuildTool = "maven"
	cfg.BuildFile = "pom.xml"

	content, err := h.readFile("pom.xml")
	if err != nil {
		return true, err
	}

	if strings.Contains(content, "spring-boot") {
		cfg.Framework = "spring-boot"
		cfg.Type = "java-spring-boot-maven"
	} else {
		cfg.Type = "java-maven"
	}

	if strings.Contains(content, "docker-maven-plugin") || strings.Contains(content, "jib-maven-plugin") {
		cfg.SkipPlugins = append(cfg.SkipPlugins, "docker-maven-plugin", "jib-maven-plugin")
		cfg.FixRequired = true
	}

	cfg.Version["java"] = h.extractMavenJavaVersion(content)
	if cfg.Version["java"] == "" {
		cfg.Version["java"] = "17"
	}
	cfg.Port = h.detectJavaPort()
	return true, nil
}

func (h *detectorHelper) detectProblematicGradlePlugins(content string) []string {
	patterns := map[string]string{
		`com\.palantir\.docker`:          "com.palantir.docker",
		`com\.bmuschko\.docker`:          "com.bmuschko.docker",
		`com\.google\.cloud\.tools\.jib`: "com.google.cloud.tools.jib",
		`gradle-docker`:                  "gradle-docker",
		`docker-compose`:                 "docker-compose",
		`nebula\.docker`:                 "nebula.docker",
	}
	var problematic []string
	for pattern, plugin := range patterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			problematic = append(problematic, plugin)
		}
	}
	return problematic
}

func (h *detectorHelper) detectGradleWrapperVersion() string {
	wrapperPath := filepath.Join(h.repoDir, "gradle", "wrapper", "gradle-wrapper.properties")
	b, err := os.ReadFile(wrapperPath)
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`gradle-(\d+\.\d+)`)
	m := re.FindStringSubmatch(string(b))
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func (h *detectorHelper) extractGradleJavaVersion(content string) string {
	patterns := []string{
		`sourceCompatibility\s*=\s*['"]?(\d+)['"]?`,
		`JavaVersion\.VERSION_(\d+)`,
		`toolchain.*languageVersion.*of\((\d+)\)`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		m := re.FindStringSubmatch(content)
		if len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func (h *detectorHelper) extractMavenJavaVersion(content string) string {
	type pom struct {
		Properties struct {
			JavaVersion         string `xml:"java.version"`
			MavenCompilerSource string `xml:"maven.compiler.source"`
			MavenCompilerTarget string `xml:"maven.compiler.target"`
		} `xml:"properties"`
	}
	var p pom
	if err := xml.Unmarshal([]byte(content), &p); err == nil {
		if p.Properties.JavaVersion != "" {
			return p.Properties.JavaVersion
		}
		if p.Properties.MavenCompilerSource != "" {
			return p.Properties.MavenCompilerSource
		}
		if p.Properties.MavenCompilerTarget != "" {
			return p.Properties.MavenCompilerTarget
		}
	}
	return ""
}

func (h *detectorHelper) detectSpringBootMainClass() string {
	var mainClass string
	_ = filepath.Walk(filepath.Join(h.repoDir, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".java") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(b)
		if strings.Contains(content, "@SpringBootApplication") {
			pkgRe := regexp.MustCompile(`package\s+([\w.]+);`)
			clsRe := regexp.MustCompile(`public\s+class\s+(\w+)`)
			pkgM := pkgRe.FindStringSubmatch(content)
			clsM := clsRe.FindStringSubmatch(content)
			if len(pkgM) > 1 && len(clsM) > 1 {
				mainClass = pkgM[1] + "." + clsM[1]
				return filepath.SkipDir
			}
		}
		return nil
	})
	return mainClass
}

func (h *detectorHelper) detectJavaPort() int {
	propsPath := filepath.Join(h.repoDir, "src", "main", "resources", "application.properties")
	if b, err := os.ReadFile(propsPath); err == nil {
		re := regexp.MustCompile(`server\.port\s*=\s*(\d+)`)
		if m := re.FindStringSubmatch(string(b)); len(m) > 1 {
			var port int
			fmt.Sscanf(m[1], "%d", &port)
			return port
		}
	}
	for _, name := range []string{"application.yml", "application.yaml"} {
		ymlPath := filepath.Join(h.repoDir, "src", "main", "resources", name)
		if b, err := os.ReadFile(ymlPath); err == nil {
			re := regexp.MustCompile(`port:\s*(\d+)`)
			if m := re.FindStringSubmatch(string(b)); len(m) > 1 {
				var port int
				fmt.Sscanf(m[1], "%d", &port)
				return port
			}
		}
	}
	return 8080
}

// ==================== NODE ====================

func (h *detectorHelper) detectNode(cfg *ProjectConfig) (bool, error) {
	if !h.hasFile("package.json") {
		return false, nil
	}
	cfg.Language = "javascript"
	cfg.Type = "node"
	cfg.BuildFile = "package.json"

	content, err := h.readFile("package.json")
	if err != nil {
		return true, err
	}
	var pkg map[string]any
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return true, err
	}

	if h.hasFile("pnpm-lock.yaml") {
		cfg.BuildTool = "pnpm"
	} else if h.hasFile("yarn.lock") {
		cfg.BuildTool = "yarn"
	} else if h.hasFile("bun.lockb") {
		cfg.BuildTool = "bun"
	} else {
		cfg.BuildTool = "npm"
	}

	if engines, ok := pkg["engines"].(map[string]any); ok {
		if node, ok := engines["node"].(string); ok {
			cfg.Version["node"] = extractVersionNumber(node)
		}
	}
	if cfg.Version["node"] == "" {
		cfg.Version["node"] = "20"
	}

	deps := extractDeps(pkg, "dependencies")
	devDeps := extractDeps(pkg, "devDependencies")

	switch {
	case deps["next"] != nil:
		cfg.Framework, cfg.Type, cfg.Port = "nextjs", "node-nextjs", 3000
	case deps["nuxt"] != nil:
		cfg.Framework, cfg.Type, cfg.Port = "nuxt", "node-nuxt", 3000
	case devDeps["vite"] != nil:
		cfg.Framework, cfg.Type, cfg.Port = "vite", "node-vite", 5173
	case deps["react-scripts"] != nil:
		cfg.Framework, cfg.Type, cfg.Port = "cra", "node-cra", 3000
	case deps["express"] != nil:
		cfg.Framework, cfg.Type, cfg.Port = "express", "node-express", 3000
	case deps["@nestjs/core"] != nil:
		cfg.Framework, cfg.Type, cfg.Port = "nestjs", "node-nestjs", 3000
	case deps["fastify"] != nil:
		cfg.Framework, cfg.Port = "fastify", 3000
	case deps["react"] != nil:
		cfg.Framework, cfg.Port = "react", 3000
	case deps["vue"] != nil:
		cfg.Framework, cfg.Port = "vue", 8080
	}

	if scripts, ok := pkg["scripts"].(map[string]any); ok {
		if v, ok := scripts["build"].(string); ok {
			cfg.BuildCommand = v
		}
		if v, ok := scripts["start"].(string); ok {
			cfg.StartCommand = v
		} else if v, ok := scripts["dev"].(string); ok {
			cfg.StartCommand = v
		}
	}
	return true, nil
}

func extractDeps(pkg map[string]any, key string) map[string]any {
	if d, ok := pkg[key].(map[string]any); ok {
		return d
	}
	return map[string]any{}
}

// ==================== PYTHON ====================

func (h *detectorHelper) detectPython(cfg *ProjectConfig) (bool, error) {
	hasPipfile := h.hasFile("Pipfile")
	hasReq := h.hasFile("requirements.txt")
	hasPyproject := h.hasFile("pyproject.toml")

	if !hasPipfile && !hasReq && !hasPyproject {
		return false, nil
	}

	cfg.Language = "python"
	cfg.Type = "python"

	if h.hasFile("runtime.txt") {
		if c, err := h.readFile("runtime.txt"); err == nil {
			cfg.Version["python"] = extractPythonVersion(c)
		}
	} else if h.hasFile(".python-version") {
		if c, err := h.readFile(".python-version"); err == nil {
			cfg.Version["python"] = strings.TrimSpace(c)
		}
	}
	if cfg.Version["python"] == "" {
		cfg.Version["python"] = "3.11"
	}

	if h.hasFile("manage.py") {
		cfg.Framework, cfg.Type, cfg.Port, cfg.BuildFile = "django", "python-django", 8000, "manage.py"
	} else if hasReq {
		content, _ := h.readFile("requirements.txt")
		switch {
		case strings.Contains(content, "fastapi"):
			cfg.Framework, cfg.Type, cfg.Port = "fastapi", "python-fastapi", 8000
		case strings.Contains(content, "flask"):
			cfg.Framework, cfg.Type, cfg.Port = "flask", "python-flask", 5000
		case strings.Contains(content, "streamlit"):
			cfg.Framework, cfg.Type, cfg.Port = "streamlit", "python-streamlit", 8501
		}
		cfg.BuildFile = "requirements.txt"
	} else if hasPyproject {
		cfg.BuildFile = "pyproject.toml"
	} else if hasPipfile {
		cfg.BuildFile = "Pipfile"
	}
	return true, nil
}

func extractPythonVersion(content string) string {
	re := regexp.MustCompile(`python-(\d+\.\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// ==================== GO ====================

func (h *detectorHelper) detectGo(cfg *ProjectConfig) (bool, error) {
	if !h.hasFile("go.mod") {
		return false, nil
	}
	cfg.Language = "go"
	cfg.Type = "go"
	cfg.BuildFile = "go.mod"
	cfg.Port = 8080

	content, err := h.readFile("go.mod")
	if err != nil {
		return true, err
	}

	re := regexp.MustCompile(`go\s+(\d+\.\d+)`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		cfg.Version["go"] = m[1]
	} else {
		cfg.Version["go"] = "1.22"
	}

	switch {
	case strings.Contains(content, "github.com/gin-gonic/gin"):
		cfg.Framework = "gin"
	case strings.Contains(content, "github.com/gofiber/fiber"):
		cfg.Framework = "fiber"
	case strings.Contains(content, "github.com/labstack/echo"):
		cfg.Framework = "echo"
	}
	return true, nil
}

// ==================== RUST ====================

func (h *detectorHelper) detectRust(cfg *ProjectConfig) (bool, error) {
	if !h.hasFile("Cargo.toml") {
		return false, nil
	}
	cfg.Language = "rust"
	cfg.Type = "rust"
	cfg.BuildFile = "Cargo.toml"
	cfg.Port = 8080

	content, err := h.readFile("Cargo.toml")
	if err != nil {
		return true, err
	}
	switch {
	case strings.Contains(content, "actix-web"):
		cfg.Framework = "actix"
	case strings.Contains(content, "rocket"):
		cfg.Framework = "rocket"
	case strings.Contains(content, "warp"):
		cfg.Framework = "warp"
	}
	return true, nil
}

// ==================== PHP ====================

func (h *detectorHelper) detectPHP(cfg *ProjectConfig) (bool, error) {
	if !h.hasFile("composer.json") && !h.hasFile("index.php") {
		return false, nil
	}
	cfg.Language = "php"
	cfg.Type = "php"
	cfg.Port = 8000

	if h.hasFile("composer.json") {
		cfg.BuildFile = "composer.json"
		content, _ := h.readFile("composer.json")
		switch {
		case strings.Contains(content, "laravel/framework"):
			cfg.Framework, cfg.Type = "laravel", "php-laravel"
		case strings.Contains(content, "symfony/symfony"):
			cfg.Framework, cfg.Type = "symfony", "php-symfony"
		}
	}

	if h.hasFile(".php-version") {
		if c, err := h.readFile(".php-version"); err == nil {
			cfg.Version["php"] = strings.TrimSpace(c)
		}
	} else {
		cfg.Version["php"] = "8.2"
	}
	return true, nil
}

// ==================== RUBY ====================

func (h *detectorHelper) detectRuby(cfg *ProjectConfig) (bool, error) {
	if !h.hasFile("Gemfile") {
		return false, nil
	}
	cfg.Language = "ruby"
	cfg.Type = "ruby"
	cfg.BuildFile = "Gemfile"
	cfg.Port = 3000

	content, _ := h.readFile("Gemfile")
	switch {
	case strings.Contains(content, "rails"):
		cfg.Framework, cfg.Type = "rails", "ruby-rails"
	case strings.Contains(content, "sinatra"):
		cfg.Framework = "sinatra"
	}

	if h.hasFile(".ruby-version") {
		if v, err := h.readFile(".ruby-version"); err == nil {
			cfg.Version["ruby"] = strings.TrimSpace(v)
		}
	} else {
		cfg.Version["ruby"] = "3.2"
	}
	return true, nil
}

// ==================== .NET ====================

func (h *detectorHelper) detectDotNet(cfg *ProjectConfig) (bool, error) {
	csproj, _ := filepath.Glob(filepath.Join(h.repoDir, "*.csproj"))
	fsproj, _ := filepath.Glob(filepath.Join(h.repoDir, "*.fsproj"))

	if len(csproj) == 0 && len(fsproj) == 0 {
		return false, nil
	}
	cfg.Language = "csharp"
	cfg.Type = "dotnet"
	cfg.Port = 5000
	cfg.Version["dotnet"] = "8.0"

	if len(csproj) > 0 {
		cfg.BuildFile = filepath.Base(csproj[0])
	} else {
		cfg.BuildFile = filepath.Base(fsproj[0])
	}
	return true, nil
}

// ==================== HELPERS ====================

func extractVersionNumber(version string) string {
	cleaned := strings.TrimSpace(version)
	for _, p := range []string{"^", "~", ">=", ">"} {
		cleaned = strings.TrimPrefix(cleaned, p)
	}
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(cleaned)
	if len(m) > 0 {
		return m[0]
	}
	return cleaned
}

func getOrDefault(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}
