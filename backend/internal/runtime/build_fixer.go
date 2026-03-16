package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BuildFixer fixes problematic build files across all languages.
type BuildFixer struct {
	logf RuntimeLogger
}

// NewBuildFixer returns a new BuildFixer. logf may be nil.
func NewBuildFixer(logf RuntimeLogger) *BuildFixer {
	if logf == nil {
		logf = func(_, _ string) {}
	}
	return &BuildFixer{logf: logf}
}

// Fix applies all necessary fixes based on project type.
func (f *BuildFixer) Fix(repoDir string, cfg *ProjectConfig) error {
	if cfg == nil {
		return nil
	}
	switch {
	case strings.Contains(cfg.Type, "gradle"):
		return f.fixGradle(repoDir)
	case strings.Contains(cfg.Type, "maven"):
		return f.fixMaven(repoDir)
	case strings.Contains(cfg.Type, "node"):
		return f.fixNode(repoDir)
	case strings.Contains(cfg.Type, "python"):
		return f.fixPython(repoDir)
	case strings.Contains(cfg.Type, "php"):
		return f.fixPHP(repoDir)
	case strings.Contains(cfg.Type, "ruby"):
		return f.fixRuby(repoDir)
	}
	return nil
}

// ==================== GRADLE ====================

func (f *BuildFixer) fixGradle(repoDir string) error {
	if err := f.fixGradleBuildFile(repoDir); err != nil {
		return err
	}
	if err := f.fixGradleSettingsFile(repoDir); err != nil {
		return err
	}
	return f.createGradleInitScript(repoDir)
}

func (f *BuildFixer) fixGradleBuildFile(repoDir string) error {
	target := ""
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(repoDir, name)); err == nil {
			target = filepath.Join(repoDir, name)
			break
		}
	}
	if target == "" {
		return nil
	}

	content, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	original := string(content)
	fixed := removeGradleProblematicPlugins(original)
	if fixed == original {
		return nil
	}

	_ = os.WriteFile(target+".backup", content, 0644)
	if err := os.WriteFile(target, []byte(fixed), 0644); err != nil {
		return err
	}
	f.logf("info", "Fixed build.gradle — removed problematic docker plugins")
	return nil
}

func removeGradleProblematicPlugins(content string) string {
	patterns := []string{
		`com\.palantir\.docker`,
		`com\.bmuschko\.docker`,
		`gradle-docker`,
		`docker-compose`,
		`com\.google\.cloud\.tools\.jib`,
		`nebula\.docker`,
	}
	lines := strings.Split(content, "\n")
	fixed := make([]string, 0, len(lines))
	for _, line := range lines {
		isProblematic := false
		for _, pat := range patterns {
			if matched, _ := regexp.MatchString(pat, line); matched {
				isProblematic = true
				break
			}
		}
		if isProblematic && !strings.HasPrefix(strings.TrimSpace(line), "//") {
			fixed = append(fixed, "// DISABLED: "+line)
		} else {
			fixed = append(fixed, line)
		}
	}
	result := strings.Join(fixed, "\n")
	re := regexp.MustCompile(`(?s)docker\s*\{[^}]*\}`)
	return re.ReplaceAllString(result, "// Docker config removed")
}

func (f *BuildFixer) fixGradleSettingsFile(repoDir string) error {
	target := ""
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(repoDir, name)); err == nil {
			target = filepath.Join(repoDir, name)
			break
		}
	}
	if target == "" {
		return nil
	}

	content, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	original := string(content)
	re := regexp.MustCompile(`(?s)pluginManagement\s*\{[^}]*docker[^}]*\}`)
	fixed := re.ReplaceAllString(original, "")
	if fixed == original {
		return nil
	}
	_ = os.WriteFile(target+".backup", content, 0644)
	return os.WriteFile(target, []byte(fixed), 0644)
}

func (f *BuildFixer) createGradleInitScript(repoDir string) error {
	initScript := `allprojects {
    afterEvaluate { project ->
        project.tasks.all { task ->
            if (task.name.toLowerCase().contains('docker')) {
                task.enabled = false
            }
        }
    }
}
`
	return os.WriteFile(filepath.Join(repoDir, "init.gradle"), []byte(initScript), 0644)
}

// ==================== MAVEN ====================

func (f *BuildFixer) fixMaven(repoDir string) error {
	pomPath := filepath.Join(repoDir, "pom.xml")
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return err
	}
	original := string(content)
	fixed := original

	for _, pat := range []string{
		`(?s)<plugin>.*?docker-maven-plugin.*?</plugin>`,
		`(?s)<plugin>.*?jib-maven-plugin.*?</plugin>`,
		`(?s)<plugin>.*?fabric8.*?</plugin>`,
	} {
		re := regexp.MustCompile(pat)
		fixed = re.ReplaceAllString(fixed, "<!-- Docker plugin removed -->")
	}
	if fixed == original {
		return nil
	}
	_ = os.WriteFile(pomPath+".backup", content, 0644)
	if err := os.WriteFile(pomPath, []byte(fixed), 0644); err != nil {
		return err
	}
	f.logf("info", "Fixed pom.xml — removed docker plugins")
	return nil
}

// ==================== NODE ====================

func (f *BuildFixer) fixNode(repoDir string) error {
	pkgPath := filepath.Join(repoDir, "package.json")
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return err
	}

	var pkg map[string]any
	if err := json.Unmarshal(content, &pkg); err != nil {
		return err
	}

	modified := false

	if engines, ok := pkg["engines"].(map[string]any); ok {
		if node, ok := engines["node"].(string); ok {
			if strings.Contains(node, "^") || strings.Contains(node, "~") {
				engines["node"] = ">=" + strings.TrimLeft(node, "^~")
				modified = true
			}
		}
	}

	if scripts, ok := pkg["scripts"].(map[string]any); ok {
		for _, s := range []string{"preinstall", "postinstall"} {
			if _, exists := scripts[s]; exists {
				delete(scripts, s)
				modified = true
			}
		}
	}

	if !modified {
		return nil
	}

	fixed, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	_ = os.WriteFile(pkgPath+".backup", content, 0644)
	if err := os.WriteFile(pkgPath, fixed, 0644); err != nil {
		return err
	}
	f.logf("info", "Fixed package.json — relaxed engine constraints")
	return nil
}

// ==================== PYTHON ====================

func (f *BuildFixer) fixPython(repoDir string) error {
	reqPath := filepath.Join(repoDir, "requirements.txt")
	if _, err := os.Stat(reqPath); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(reqPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	fixed := make([]string, 0, len(lines))
	modified := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			fixed = append(fixed, line)
			continue
		}
		if strings.Contains(trimmed, "==") {
			parts := strings.SplitN(trimmed, "==", 2)
			fixed = append(fixed, parts[0]+">="+parts[1])
			modified = true
		} else {
			fixed = append(fixed, line)
		}
	}

	if !modified {
		return nil
	}

	_ = os.WriteFile(reqPath+".backup", content, 0644)
	if err := os.WriteFile(reqPath, []byte(strings.Join(fixed, "\n")), 0644); err != nil {
		return err
	}
	f.logf("info", "Fixed requirements.txt — relaxed pinned versions")
	return nil
}

// ==================== PHP ====================

func (f *BuildFixer) fixPHP(repoDir string) error {
	composerPath := filepath.Join(repoDir, "composer.json")
	if _, err := os.Stat(composerPath); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(composerPath)
	if err != nil {
		return err
	}

	var composer map[string]any
	if err := json.Unmarshal(content, &composer); err != nil {
		return err
	}

	modified := false
	if require, ok := composer["require"].(map[string]any); ok {
		if php, ok := require["php"].(string); ok {
			if strings.Contains(php, "^") {
				require["php"] = ">=" + strings.TrimPrefix(php, "^")
				modified = true
			}
		}
	}

	if !modified {
		return nil
	}

	fixed, err := json.MarshalIndent(composer, "", "  ")
	if err != nil {
		return err
	}
	_ = os.WriteFile(composerPath+".backup", content, 0644)
	return os.WriteFile(composerPath, fixed, 0644)
}

// ==================== RUBY ====================

func (f *BuildFixer) fixRuby(repoDir string) error {
	gemfilePath := filepath.Join(repoDir, "Gemfile")
	if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {
		return nil
	}

	content, err := os.ReadFile(gemfilePath)
	if err != nil {
		return err
	}

	original := string(content)
	re := regexp.MustCompile(`ruby ['"]~>([^'"]+)['"]`)
	fixed := re.ReplaceAllString(original, "ruby '>=$1'")

	if fixed == original {
		return nil
	}
	_ = os.WriteFile(gemfilePath+".backup", content, 0644)
	if err := os.WriteFile(gemfilePath, []byte(fixed), 0644); err != nil {
		return err
	}
	f.logf("info", fmt.Sprintf("Fixed Gemfile — relaxed Ruby version constraint"))
	return nil
}
