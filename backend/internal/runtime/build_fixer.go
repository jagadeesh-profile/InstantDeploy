package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type RuntimeLogger func(level, message string)

type BuildFixer struct {
	logf RuntimeLogger
}

func NewBuildFixer(logf RuntimeLogger) *BuildFixer {
	return &BuildFixer{logf: logf}
}

func (f *BuildFixer) Fix(repoDir string, project *DetectedProject) error {
	if project == nil {
		return nil
	}

	switch project.Kind {
	case ProjectKindJavaGradle:
		return f.fixGradle(repoDir)
	case ProjectKindJavaMaven:
		return f.fixMaven(repoDir)
	case ProjectKindNode:
		return f.fixNode(repoDir, project)
	case ProjectKindPython:
		return f.fixPython(repoDir, project)
	case ProjectKindPHP:
		return f.fixPHP(repoDir, project)
	case ProjectKindRuby:
		return f.fixRuby(repoDir, project)
	default:
		return nil
	}
}

func (f *BuildFixer) fixGradle(repoDir string) error {
	files := []string{"build.gradle", "build.gradle.kts"}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?im)^.*com\.palantir\.docker.*$`),
		regexp.MustCompile(`(?im)^.*com\.google\.cloud\.tools\.jib.*$`),
		regexp.MustCompile(`(?im)^.*com\.bmuschko\.docker.*$`),
		regexp.MustCompile(`(?im)^.*bootBuildImage.*$`),
		regexp.MustCompile(`(?im)^.*jib\s*\{.*$`),
	}

	for _, name := range files {
		path := filepath.Join(repoDir, name)
		if !fileExists(path) {
			continue
		}
		originalBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", name, err)
		}
		original := string(originalBytes)
		updated := original
		for _, pattern := range patterns {
			updated = pattern.ReplaceAllStringFunc(updated, func(line string) string {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "// instantdeploy:") {
					return line
				}
				return "// instantdeploy: disabled incompatible docker plugin/task -> " + trimmed
			})
		}
		if updated != original {
			if err := f.backupAndWrite(path, updated); err != nil {
				return err
			}
			f.log("warn", fmt.Sprintf("Patched %s to disable incompatible Docker-oriented Gradle plugins/tasks", name))
		}
	}

	initScript := `allprojects {
	gradle.taskGraph.whenReady { graph ->
		tasks.each { task ->
			def lowered = task.name.toLowerCase()
			if (lowered.startsWith("docker") || lowered.startsWith("jib") || lowered == "bootbuildimage") {
				task.enabled = false
			}
		}
	}
}
`
	initPath := filepath.Join(repoDir, ".instantdeploy.init.gradle")
	if err := os.WriteFile(initPath, []byte(initScript), 0o644); err != nil {
		return fmt.Errorf("failed to write init.gradle: %w", err)
	}
	f.log("info", "Prepared Gradle init script to disable Docker-specific tasks during builds")
	return nil
}

func (f *BuildFixer) fixMaven(repoDir string) error {
	path := filepath.Join(repoDir, "pom.xml")
	if !fileExists(path) {
		return nil
	}

	originalBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read pom.xml: %w", err)
	}
	original := string(originalBytes)
	updated := original
	pluginPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?is)<plugin>\s*<groupId>com\.google\.cloud\.tools</groupId>\s*<artifactId>jib-maven-plugin</artifactId>.*?</plugin>`),
		regexp.MustCompile(`(?is)<plugin>\s*<groupId>io\.fabric8</groupId>\s*<artifactId>docker-maven-plugin</artifactId>.*?</plugin>`),
		regexp.MustCompile(`(?is)<plugin>\s*<groupId>com\.spotify</groupId>\s*<artifactId>dockerfile-maven-plugin</artifactId>.*?</plugin>`),
	}
	for _, pattern := range pluginPatterns {
		updated = pattern.ReplaceAllString(updated, "<!-- instantdeploy: removed incompatible docker plugin -->")
	}
	if updated != original {
		if err := f.backupAndWrite(path, updated); err != nil {
			return err
		}
		f.log("warn", "Patched pom.xml to remove incompatible Docker build plugins")
	}
	return nil
}

func (f *BuildFixer) fixNode(repoDir string, project *DetectedProject) error {
	path := filepath.Join(repoDir, "package.json")
	if !fileExists(path) {
		return nil
	}

	originalBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg map[string]any
	if err := json.Unmarshal(originalBytes, &pkg); err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	changed := false
	if engines, ok := pkg["engines"].(map[string]any); ok {
		if node, ok := engines["node"].(string); ok && strings.Contains(node, "||") {
			engines["node"] = strings.TrimSpace(strings.Split(node, "||")[0])
			changed = true
		}
	}

	if scripts, ok := pkg["scripts"].(map[string]any); ok {
		for _, key := range []string{"preinstall", "postinstall"} {
			if script, exists := scripts[key].(string); exists {
				lowered := strings.ToLower(script)
				if strings.Contains(lowered, "docker") || strings.Contains(lowered, "buildkit") {
					delete(scripts, key)
					changed = true
				}
			}
		}
	}

	if changed {
		encoded, err := json.MarshalIndent(pkg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to write fixed package.json: %w", err)
		}
		if err := f.backupAndWrite(path, string(encoded)+"\n"); err != nil {
			return err
		}
		f.log("warn", "Patched package.json to avoid install hooks that commonly fail in container builds")
	}

	if project != nil && project.PackageManager == "pnpm" && !fileExists(filepath.Join(repoDir, "pnpm-lock.yaml")) {
		f.log("warn", "pnpm project detected without lockfile; build will fall back to a non-frozen install")
	}
	return nil
}

func (f *BuildFixer) fixPython(repoDir string, project *DetectedProject) error {
	reqPath := filepath.Join(repoDir, "requirements.txt")
	if fileExists(reqPath) {
		originalBytes, err := os.ReadFile(reqPath)
		if err != nil {
			return fmt.Errorf("failed to read requirements.txt: %w", err)
		}
		original := string(originalBytes)
		lines := strings.Split(original, "\n")
		changed := false
		for i := range lines {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "git+") {
				continue
			}
			if strings.Contains(line, "==") {
				lines[i] = strings.Replace(line, "==", ">=", 1)
				changed = true
			}
		}
		if changed {
			if err := f.backupAndWrite(reqPath, strings.Join(lines, "\n")); err != nil {
				return err
			}
			f.log("warn", "Relaxed pinned requirements in requirements.txt to reduce dependency resolution failures")
		}
	}

	pyprojectPath := filepath.Join(repoDir, "pyproject.toml")
	if fileExists(pyprojectPath) {
		originalBytes, err := os.ReadFile(pyprojectPath)
		if err != nil {
			return fmt.Errorf("failed to read pyproject.toml: %w", err)
		}
		original := string(originalBytes)
		updated := strings.ReplaceAll(original, "==", ">=")
		if updated != original {
			if err := f.backupAndWrite(pyprojectPath, updated); err != nil {
				return err
			}
			f.log("warn", "Relaxed exact dependency constraints in pyproject.toml")
		}
	}

	_ = project
	return nil
}

func (f *BuildFixer) fixPHP(repoDir string, project *DetectedProject) error {
	path := filepath.Join(repoDir, "composer.json")
	if !fileExists(path) {
		_ = project
		return nil
	}

	originalBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read composer.json: %w", err)
	}

	var composer map[string]any
	if err := json.Unmarshal(originalBytes, &composer); err != nil {
		return fmt.Errorf("failed to parse composer.json: %w", err)
	}

	changed := false
	if req, ok := composer["require"].(map[string]any); ok {
		if php, ok := req["php"].(string); ok {
			php = strings.TrimSpace(php)
			if strings.HasPrefix(php, "=") {
				req["php"] = ">=" + strings.TrimPrefix(php, "=")
				changed = true
			}
		}
	}

	if changed {
		encoded, err := json.MarshalIndent(composer, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to write fixed composer.json: %w", err)
		}
		if err := f.backupAndWrite(path, string(encoded)+"\n"); err != nil {
			return err
		}
		f.log("warn", "Relaxed PHP runtime version constraint in composer.json")
	}

	_ = project
	return nil
}

func (f *BuildFixer) fixRuby(repoDir string, project *DetectedProject) error {
	gemfilePath := filepath.Join(repoDir, "Gemfile")
	if fileExists(gemfilePath) {
		originalBytes, err := os.ReadFile(gemfilePath)
		if err != nil {
			return fmt.Errorf("failed to read Gemfile: %w", err)
		}
		original := string(originalBytes)
		// Convert strict `ruby "x.y.z"` declarations to a compatible lower bound.
		re := regexp.MustCompile(`(?m)^\s*ruby\s+['"]([0-9]+\.[0-9]+(?:\.[0-9]+)?)['"]\s*$`)
		updated := re.ReplaceAllString(original, `ruby ">= $1"`)
		if updated != original {
			if err := f.backupAndWrite(gemfilePath, updated); err != nil {
				return err
			}
			f.log("warn", "Relaxed strict Ruby version declaration in Gemfile")
		}
	}

	rubyVersionPath := filepath.Join(repoDir, ".ruby-version")
	if fileExists(rubyVersionPath) {
		version := strings.TrimSpace(readOptional(rubyVersionPath))
		if strings.Count(version, ".") >= 2 {
			parts := strings.Split(version, ".")
			if len(parts) >= 2 {
				relaxed := parts[0] + "." + parts[1]
				if relaxed != version {
					if err := f.backupAndWrite(rubyVersionPath, relaxed+"\n"); err != nil {
						return err
					}
					f.log("warn", "Relaxed .ruby-version patch level to improve image compatibility")
				}
			}
		}
	}

	_ = project
	return nil
}

func (f *BuildFixer) backupAndWrite(path, updated string) error {
	backupPath := path + ".instantdeploy.bak"
	if !fileExists(backupPath) {
		originalBytes, err := os.ReadFile(path)
		if err == nil {
			if err := os.WriteFile(backupPath, originalBytes, 0o644); err != nil {
				return fmt.Errorf("failed to write backup for %s: %w", filepath.Base(path), err)
			}
		}
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func (f *BuildFixer) log(level, message string) {
	if f.logf != nil {
		f.logf(level, message)
	}
}
