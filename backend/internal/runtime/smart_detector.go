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



// ProjectConfig holds comprehensive project configuration

type ProjectConfig struct {

    Type            string                 `json:"type"`

    Language        string                 `json:"language"`

    Framework       string                 `json:"framework"`

    BuildTool       string                 `json:"build_tool"`

    Version         map[string]string      `json:"version"`

    BuildFile       string                 `json:"build_file"`

    MainClass       string                 `json:"main_class"`

    Port             int                    `json:"port"`

    BuildCommand    string                 `json:"build_command"`

    StartCommand    string                 `json:"start_command"`

    EnvVars         map[string]string      `json:"env_vars"`

    SkipPlugins     []string               `json:"skip_plugins"`

    FixRequired     bool                   `json:"fix_required"`

    CustomSettings  map[string]interface{} `json:"custom_settings"`

}



// SmartDetector intelligently detects and analyzes projects

type SmartDetector struct {

    repoDir string

}



func NewSmartDetector(repoDir string) *SmartDetector {

    return &SmartDetector{repoDir: repoDir}

}



// Detect performs comprehensive project detection

func (d *SmartDetector) Detect() (*ProjectConfig, error) {

    config := &ProjectConfig{

        Version:        make(map[string]string),

        EnvVars:        make(map[string]string),

        CustomSettings: make(map[string]interface{}),

        SkipPlugins:    []string{},

        Port:           8080,

        FixRequired:    false,

    }



    // Priority order: Custom Dockerfile  Java  Node  Python  Go  Rust  Static



    // 1. Check for custom Dockerfile

    if d.hasFile("Dockerfile") {

        config.Type = "custom"

        config.Language = "docker"

        return config, nil

    }



    // 2. Detect Java projects (Gradle/Maven)

    if detected, err := d.detectJava(config); detected {

        return config, err

    }



    // 3. Detect Node.js projects

    if detected, err := d.detectNode(config); detected {

        return config, err

    }



    // 4. Detect Python projects

    if detected, err := d.detectPython(config); detected {

        return config, err

    }



    // 5. Detect Go projects

    if detected, err := d.detectGo(config); detected {

        return config, err

    }



    // 6. Detect Rust projects

    if detected, err := d.detectRust(config); detected {

        return config, err

    }



    // 7. Detect PHP projects

    if detected, err := d.detectPHP(config); detected {

        return config, err

    }



    // 8. Detect Ruby projects

    if detected, err := d.detectRuby(config); detected {

        return config, err

    }



    // 9. Detect .NET projects

    if detected, err := d.detectDotNet(config); detected {

        return config, err

    }



    // 10. Default to static

    config.Type = "static"

    config.Language = "html"

    config.Port = 80

    return config, nil

}



// ==================== JAVA DETECTION ====================



func (d *SmartDetector) detectJava(config *ProjectConfig) (bool, error) {

    // Check for Gradle

    if d.hasFile("build.gradle") || d.hasFile("build.gradle.kts") {

        return d.detectGradle(config)

    }



    // Check for Maven

    if d.hasFile("pom.xml") {

        return d.detectMaven(config)

    }



    return false, nil

}



func (d *SmartDetector) detectGradle(config *ProjectConfig) (bool, error) {

    config.Language = "java"

    config.BuildTool = "gradle"



    buildFile := "build.gradle"

    if d.hasFile("build.gradle.kts") {

        buildFile = "build.gradle.kts"

    }

    config.BuildFile = buildFile



    content, err := d.readFile(buildFile)

    if err != nil {

        return true, err

    }



    // Detect Spring Boot

    if strings.Contains(content, "spring-boot") || strings.Contains(content, "org.springframework.boot") {

        config.Framework = "spring-boot"

        config.Type = "java-spring-boot-gradle"

    } else {

        config.Type = "java-gradle"

    }



    // Detect problematic plugins (CRITICAL!)

    config.SkipPlugins = d.detectProblematicGradlePlugins(content)

    if len(config.SkipPlugins) > 0 {

        config.FixRequired = true

    }



    // Detect Java version

    config.Version["java"] = d.extractGradleJavaVersion(content)

    if config.Version["java"] == "" {

        config.Version["java"] = "17"

    }



    // Detect Gradle version

    config.Version["gradle"] = d.detectGradleWrapperVersion()

    if config.Version["gradle"] == "" {

        config.Version["gradle"] = "8.5"

    }



    // Detect main class

    config.MainClass = d.detectSpringBootMainClass()



    // Detect port

    config.Port = d.detectJavaPort()



    return true, nil

}



func (d *SmartDetector) detectMaven(config *ProjectConfig) (bool, error) {

    config.Language = "java"

    config.BuildTool = "maven"

    config.BuildFile = "pom.xml"



    content, err := d.readFile("pom.xml")

    if err != nil {

        return true, err

    }



    // Detect Spring Boot

    if strings.Contains(content, "spring-boot") {

        config.Framework = "spring-boot"

        config.Type = "java-spring-boot-maven"

    } else {

        config.Type = "java-maven"

    }



    // Check for problematic plugins

    if strings.Contains(content, "docker-maven-plugin") ||

        strings.Contains(content, "jib-maven-plugin") {

        config.SkipPlugins = append(config.SkipPlugins, "docker-maven-plugin", "jib-maven-plugin")

        config.FixRequired = true

    }



    // Parse Java version

    config.Version["java"] = d.extractMavenJavaVersion(content)

    if config.Version["java"] == "" {

        config.Version["java"] = "17"

    }



    config.Port = d.detectJavaPort()



    return true, nil

}



// Detect problematic Gradle plugins

func (d *SmartDetector) detectProblematicGradlePlugins(content string) []string {

    problematic := []string{}



    patterns := map[string]string{

        `com\.palantir\.docker`:        "com.palantir.docker",

        `com\.bmuschko\.docker`:        "com.bmuschko.docker",

        `com\.google\.cloud\.tools\.jib`: "com.google.cloud.tools.jib",

        `gradle-docker`:                "gradle-docker",

        `docker-compose`:               "docker-compose",

        `nebula\.docker`:               "nebula.docker",

    }



    for pattern, plugin := range patterns {

        if matched, _ := regexp.MatchString(pattern, content); matched {

            problematic = append(problematic, plugin)

        }

    }



    return problematic

}



func (d *SmartDetector) detectGradleWrapperVersion() string {

    wrapperPath := filepath.Join(d.repoDir, "gradle", "wrapper", "gradle-wrapper.properties")

    content, err := os.ReadFile(wrapperPath)

    if err != nil {

        return ""

    }



    re := regexp.MustCompile(`gradle-(\d+\.\d+)`)

    matches := re.FindStringSubmatch(string(content))

    if len(matches) > 1 {

        return matches[1]

    }

    return ""

}



func (d *SmartDetector) extractGradleJavaVersion(content string) string {

    patterns := []string{

        `sourceCompatibility\s*=\s*['"]?(\d+)['"]?`,

        `JavaVersion\.VERSION_(\d+)`,

        `toolchain.*languageVersion.*of\((\d+)\)`,

        `java\s*{\s*sourceCompatibility\s*=\s*['"]?(\d+)['"]?`,

    }



    for _, pattern := range patterns {

        re := regexp.MustCompile(pattern)

        matches := re.FindStringSubmatch(content)

        if len(matches) > 1 {

            return matches[1]

        }

    }

    return ""

}



func (d *SmartDetector) extractMavenJavaVersion(content string) string {

    type POM struct {

        Properties struct {

            JavaVersion         string `xml:"java.version"`

            MavenCompilerSource string `xml:"maven.compiler.source"`

            MavenCompilerTarget string `xml:"maven.compiler.target"`

        } `xml:"properties"`

    }



    var pom POM

    if err := xml.Unmarshal([]byte(content), &pom); err == nil {

        if pom.Properties.JavaVersion != "" {

            return pom.Properties.JavaVersion

        }

        if pom.Properties.MavenCompilerSource != "" {

            return pom.Properties.MavenCompilerSource

        }

        if pom.Properties.MavenCompilerTarget != "" {

            return pom.Properties.MavenCompilerTarget

        }

    }

    return ""

}



func (d *SmartDetector) detectSpringBootMainClass() string {

    var mainClass string



    filepath.Walk(filepath.Join(d.repoDir, "src"), func(path string, info os.FileInfo, err error) error {

        if err != nil || info.IsDir() || !strings.HasSuffix(path, ".java") {

            return nil

        }



        content, err := os.ReadFile(path)

        if err != nil {

            return nil

        }



        if strings.Contains(string(content), "@SpringBootApplication") {

            packageRe := regexp.MustCompile(`package\s+([\w.]+);`)

            classRe := regexp.MustCompile(`public\s+class\s+(\w+)`)



            packageMatch := packageRe.FindStringSubmatch(string(content))

            classMatch := classRe.FindStringSubmatch(string(content))



            if len(packageMatch) > 1 && len(classMatch) > 1 {

                mainClass = packageMatch[1] + "." + classMatch[1]

                return filepath.SkipDir

            }

        }

        return nil

    })



    return mainClass

}



func (d *SmartDetector) detectJavaPort() int {

    // Check application.properties

    propsPath := filepath.Join(d.repoDir, "src", "main", "resources", "application.properties")

    if content, err := os.ReadFile(propsPath); err == nil {

        re := regexp.MustCompile(`server\.port\s*=\s*(\d+)`)

        if matches := re.FindStringSubmatch(string(content)); len(matches) > 1 {

            var port int

            fmt.Sscanf(matches[1], "%d", &port)

            return port

        }

    }



    // Check application.yml/yaml

    for _, name := range []string{"application.yml", "application.yaml"} {

        ymlPath := filepath.Join(d.repoDir, "src", "main", "resources", name)

        if content, err := os.ReadFile(ymlPath); err == nil {

            re := regexp.MustCompile(`port:\s*(\d+)`)

            if matches := re.FindStringSubmatch(string(content)); len(matches) > 1 {

                var port int

                fmt.Sscanf(matches[1], "%d", &port)

                return port

            }

        }

    }



    return 8080 // Default Spring Boot port

}



// ==================== NODE.JS DETECTION ====================



func (d *SmartDetector) detectNode(config *ProjectConfig) (bool, error) {

    if !d.hasFile("package.json") {

        return false, nil

    }



    config.Language = "javascript"

    config.Type = "node"

    config.BuildFile = "package.json"



    content, err := d.readFile("package.json")

    if err != nil {

        return true, err

    }



    var pkg map[string]interface{}

    if err := json.Unmarshal([]byte(content), &pkg); err != nil {

        return true, err

    }



    // Detect package manager

    if d.hasFile("pnpm-lock.yaml") {

        config.BuildTool = "pnpm"

    } else if d.hasFile("yarn.lock") {

        config.BuildTool = "yarn"

    } else if d.hasFile("bun.lockb") {

        config.BuildTool = "bun"

    } else {

        config.BuildTool = "npm"

    }



    // Detect Node version

    if engines, ok := pkg["engines"].(map[string]interface{}); ok {

        if node, ok := engines["node"].(string); ok {

            config.Version["node"] = d.extractVersionNumber(node)

        }

    }

    if config.Version["node"] == "" {

        config.Version["node"] = "20" // Latest LTS

    }



    // Detect framework

    deps := d.extractDependencies(pkg, "dependencies")

    devDeps := d.extractDependencies(pkg, "devDependencies")



    // Next.js

    if _, hasNext := deps["next"]; hasNext {

        config.Framework = "nextjs"

        config.Type = "node-nextjs"

        config.Port = 3000

    // Nuxt.js

    } else if _, hasNuxt := deps["nuxt"]; hasNuxt {

        config.Framework = "nuxt"

        config.Type = "node-nuxt"

        config.Port = 3000

    // Vite + React/Vue

    } else if _, hasVite := devDeps["vite"]; hasVite {

        config.Framework = "vite"

        config.Type = "node-vite"

        config.Port = 5173

    // Create React App

    } else if _, hasCRA := deps["react-scripts"]; hasCRA {

        config.Framework = "cra"

        config.Type = "node-cra"

        config.Port = 3000

    // React (generic)

    } else if _, hasReact := deps["react"]; hasReact {

        config.Framework = "react"

        config.Port = 3000

    // Vue (generic)

    } else if _, hasVue := deps["vue"]; hasVue {

        config.Framework = "vue"

        config.Port = 8080

    // Express

    } else if _, hasExpress := deps["express"]; hasExpress {

        config.Framework = "express"

        config.Type = "node-express"

        config.Port = 3000

    // NestJS

    } else if _, hasNest := deps["@nestjs/core"]; hasNest {

        config.Framework = "nestjs"

        config.Type = "node-nestjs"

        config.Port = 3000

    // Fastify

    } else if _, hasFastify := deps["fastify"]; hasFastify {

        config.Framework = "fastify"

        config.Port = 3000

    }



    // Extract build and start scripts

    if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {

        if build, ok := scripts["build"].(string); ok {

            config.BuildCommand = build

        }

        if start, ok := scripts["start"].(string); ok {

            config.StartCommand = start

        }

        if dev, ok := scripts["dev"].(string); ok && config.StartCommand == "" {

            config.StartCommand = dev

        }

    }



    return true, nil

}



func (d *SmartDetector) extractDependencies(pkg map[string]interface{}, key string) map[string]interface{} {

    if deps, ok := pkg[key].(map[string]interface{}); ok {

        return deps

    }

    return make(map[string]interface{})

}



// ==================== PYTHON DETECTION ====================



func (d *SmartDetector) detectPython(config *ProjectConfig) (bool, error) {

    hasPipfile := d.hasFile("Pipfile")

    hasRequirements := d.hasFile("requirements.txt")

    hasPyproject := d.hasFile("pyproject.toml")



    if !hasPipfile && !hasRequirements && !hasPyproject {

        return false, nil

    }



    config.Language = "python"

    config.Type = "python"



    // Detect Python version

    if d.hasFile("runtime.txt") {

        content, _ := d.readFile("runtime.txt")

        config.Version["python"] = d.extractPythonVersion(content)

    } else if d.hasFile(".python-version") {

        content, _ := d.readFile(".python-version")

        config.Version["python"] = strings.TrimSpace(content)

    }



    if config.Version["python"] == "" {

        config.Version["python"] = "3.11"

    }



    // Detect framework

    if d.hasFile("manage.py") {

        config.Framework = "django"

        config.Type = "python-django"

        config.Port = 8000

        config.BuildFile = "manage.py"

    } else if hasRequirements {

        content, _ := d.readFile("requirements.txt")

        if strings.Contains(content, "flask") {

            config.Framework = "flask"

            config.Type = "python-flask"

            config.Port = 5000

        } else if strings.Contains(content, "fastapi") {

            config.Framework = "fastapi"

            config.Type = "python-fastapi"

            config.Port = 8000

        } else if strings.Contains(content, "streamlit") {

            config.Framework = "streamlit"

            config.Type = "python-streamlit"

            config.Port = 8501

        }

        config.BuildFile = "requirements.txt"

    } else if hasPyproject {

        config.BuildFile = "pyproject.toml"

    } else if hasPipfile {

        config.BuildFile = "Pipfile"

    }



    return true, nil

}



func (d *SmartDetector) extractPythonVersion(content string) string {

    re := regexp.MustCompile(`python-(\d+\.\d+(?:\.\d+)?)`)

    matches := re.FindStringSubmatch(content)

    if len(matches) > 1 {

        return matches[1]

    }

    return ""

}



// ==================== GO DETECTION ====================



func (d *SmartDetector) detectGo(config *ProjectConfig) (bool, error) {

    if !d.hasFile("go.mod") {

        return false, nil

    }



    config.Language = "go"

    config.Type = "go"

    config.BuildFile = "go.mod"

    config.Port = 8080



    content, err := d.readFile("go.mod")

    if err != nil {

        return true, err

    }



    // Extract Go version

    re := regexp.MustCompile(`go\s+(\d+\.\d+)`)

    matches := re.FindStringSubmatch(content)

    if len(matches) > 1 {

        config.Version["go"] = matches[1]

    } else {

        config.Version["go"] = "1.21"

    }



    // Detect frameworks

    if strings.Contains(content, "github.com/gin-gonic/gin") {

        config.Framework = "gin"

    } else if strings.Contains(content, "github.com/gofiber/fiber") {

        config.Framework = "fiber"

    } else if strings.Contains(content, "github.com/labstack/echo") {

        config.Framework = "echo"

    }



    return true, nil

}



// ==================== RUST DETECTION ====================



func (d *SmartDetector) detectRust(config *ProjectConfig) (bool, error) {

    if !d.hasFile("Cargo.toml") {

        return false, nil

    }



    config.Language = "rust"

    config.Type = "rust"

    config.BuildFile = "Cargo.toml"

    config.Port = 8080



    content, err := d.readFile("Cargo.toml")

    if err != nil {

        return true, err

    }



    // Detect frameworks

    if strings.Contains(content, "actix-web") {

        config.Framework = "actix"

    } else if strings.Contains(content, "rocket") {

        config.Framework = "rocket"

    } else if strings.Contains(content, "warp") {

        config.Framework = "warp"

    }



    return true, nil

}



// ==================== PHP DETECTION ====================



func (d *SmartDetector) detectPHP(config *ProjectConfig) (bool, error) {

    if !d.hasFile("composer.json") && !d.hasFile("index.php") {

        return false, nil

    }



    config.Language = "php"

    config.Type = "php"

    config.Port = 8000



    if d.hasFile("composer.json") {

        config.BuildFile = "composer.json"

        content, _ := d.readFile("composer.json")



        if strings.Contains(content, "laravel/framework") {

            config.Framework = "laravel"

            config.Type = "php-laravel"

        } else if strings.Contains(content, "symfony/symfony") {

            config.Framework = "symfony"

            config.Type = "php-symfony"

        }

    }



    // Detect PHP version

    if d.hasFile(".php-version") {

        content, _ := d.readFile(".php-version")

        config.Version["php"] = strings.TrimSpace(content)

    } else {

        config.Version["php"] = "8.2"

    }



    return true, nil

}



// ==================== RUBY DETECTION ====================



func (d *SmartDetector) detectRuby(config *ProjectConfig) (bool, error) {

    if !d.hasFile("Gemfile") {

        return false, nil

    }



    config.Language = "ruby"

    config.Type = "ruby"

    config.BuildFile = "Gemfile"

    config.Port = 3000



    content, _ := d.readFile("Gemfile")



    if strings.Contains(content, "rails") {

        config.Framework = "rails"

        config.Type = "ruby-rails"

    } else if strings.Contains(content, "sinatra") {

        config.Framework = "sinatra"

    }



    // Detect Ruby version

    if d.hasFile(".ruby-version") {

        version, _ := d.readFile(".ruby-version")

        config.Version["ruby"] = strings.TrimSpace(version)

    } else {

        config.Version["ruby"] = "3.2"

    }



    return true, nil

}



// ==================== .NET DETECTION ====================



func (d *SmartDetector) detectDotNet(config *ProjectConfig) (bool, error) {

    // Check for .csproj or .fsproj files

    csprojFiles, _ := filepath.Glob(filepath.Join(d.repoDir, "*.csproj"))

    fsprojFiles, _ := filepath.Glob(filepath.Join(d.repoDir, "*.fsproj"))



    if len(csprojFiles) == 0 && len(fsprojFiles) == 0 {

        return false, nil

    }



    config.Language = "csharp"

    config.Type = "dotnet"

    config.Port = 5000



    if len(csprojFiles) > 0 {

        config.BuildFile = filepath.Base(csprojFiles[0])

    } else {

        config.BuildFile = filepath.Base(fsprojFiles[0])

    }



    // Detect .NET version

    config.Version["dotnet"] = "8.0"



    return true, nil

}



// ==================== HELPER METHODS ====================



func (d *SmartDetector) hasFile(filename string) bool {

    _, err := os.Stat(filepath.Join(d.repoDir, filename))

    return err == nil

}



func (d *SmartDetector) readFile(filename string) (string, error) {

    content, err := os.ReadFile(filepath.Join(d.repoDir, filename))

    if err != nil {

        return "", err

    }

    return string(content), nil

}



func (d *SmartDetector) extractVersionNumber(version string) string {

    // Remove ^, ~, >=, etc.

    cleaned := strings.TrimSpace(version)

    cleaned = strings.TrimPrefix(cleaned, "^")

    cleaned = strings.TrimPrefix(cleaned, "~")

    cleaned = strings.TrimPrefix(cleaned, ">=")

    cleaned = strings.TrimPrefix(cleaned, ">")



    re := regexp.MustCompile(`(\d+(?:\.\d+)?)`)

    matches := re.FindStringSubmatch(cleaned)

    if len(matches) > 0 {

        return matches[0]

    }

    return cleaned

}
