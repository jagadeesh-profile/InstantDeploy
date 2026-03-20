package runtime

import (
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
	// Avoid mutating package.json automatically. Script and engine rewrites can
	// break legitimate build flows and are a common source of false failures.
	_ = repoDir
	f.logf("info", "Skipped package.json mutation for safer build compatibility")
	return nil
}

// ==================== PYTHON ====================

func (f *BuildFixer) fixPython(repoDir string) error {
	// Do not rewrite dependency pins automatically. Many repos depend on exact
	// versions for compatibility and relaxing versions increases build failures.
	_ = repoDir
	f.logf("info", "Skipped dependency rewrite for Python compatibility")
	return nil
}

// ==================== PHP ====================

func (f *BuildFixer) fixPHP(repoDir string) error {
	_ = repoDir
	f.logf("info", "Skipped composer.json mutation for safer PHP compatibility")
	return nil
}

// ==================== RUBY ====================

func (f *BuildFixer) fixRuby(repoDir string) error {
	_ = repoDir
	f.logf("info", "Skipped Gemfile mutation for safer Ruby compatibility")
	return nil
}
