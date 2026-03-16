package runtime



import (

    "fmt"

    "os"

    "path/filepath"

    "regexp"

    "strings"

)



// BuildFixer fixes problematic build files across all languages

type BuildFixer struct {

    repoDir string

    config  *ProjectConfig

}



func NewBuildFixer(repoDir string, config *ProjectConfig) *BuildFixer {

    return &BuildFixer{

        repoDir: repoDir,

        config:  config,

    }

}



// Fix applies all necessary fixes based on project type

func (f *BuildFixer) Fix() error {

    switch {

    case strings.Contains(f.config.Type, "gradle"):

        return f.fixGradle()

    case strings.Contains(f.config.Type, "maven"):

        return f.fixMaven()

    case strings.Contains(f.config.Type, "node"):

        return f.fixNode()

    case strings.Contains(f.config.Type, "python"):

        return f.fixPython()

    case strings.Contains(f.config.Type, "php"):

        return f.fixPHP()

    case strings.Contains(f.config.Type, "ruby"):

        return f.fixRuby()

    }

    return nil

}



// ==================== GRADLE FIXES ====================



func (f *BuildFixer) fixGradle() error {

    // Fix build.gradle

    if err := f.fixGradleBuildFile(); err != nil {

        return err

    }



    // Fix settings.gradle

    if err := f.fixGradleSettingsFile(); err != nil {

        return err

    }



    // Create init.gradle override

    if err := f.createGradleInitScript(); err != nil {

        return err

    }



    return nil

}



func (f *BuildFixer) fixGradleBuildFile() error {

    buildFile := filepath.Join(f.repoDir, "build.gradle")

    buildFileKts := filepath.Join(f.repoDir, "build.gradle.kts")



    targetFile := ""

    if _, err := os.Stat(buildFile); err == nil {

        targetFile = buildFile

    } else if _, err := os.Stat(buildFileKts); err == nil {

        targetFile = buildFileKts

    } else {

        return nil

    }



    content, err := os.ReadFile(targetFile)

    if err != nil {

        return err

    }



    original := string(content)

    fixed := f.removeGradleProblematicPlugins(original)



    if fixed != original {

        // Backup original

        os.WriteFile(targetFile+".backup", content, 0644)

        

        // Write fixed version

        if err := os.WriteFile(targetFile, []byte(fixed), 0644); err != nil {

            return err

        }

        

        fmt.Println(" Fixed build.gradle - removed problematic plugins")

    }



    return nil

}



func (f *BuildFixer) removeGradleProblematicPlugins(content string) string {

    lines := strings.Split(content, "\n")

    fixedLines := []string{}



    problematicPatterns := []string{

        `com\.palantir\.docker`,

        `com\.bmuschko\.docker`,

        `gradle-docker`,

        `docker-compose`,

        `com\.google\.cloud\.tools\.jib`,

        `nebula\.docker`,

        `io\.spring\.dependency-management`, // Sometimes problematic

    }



    for _, line := range lines {

        isProblematic := false

        

        for _, pattern := range problematicPatterns {

            if matched, _ := regexp.MatchString(pattern, line); matched {

                isProblematic = true

                break

            }

        }



        if isProblematic {

            if !strings.HasPrefix(strings.TrimSpace(line), "//") {

                fixedLines = append(fixedLines, "// DISABLED: "+line)

            } else {

                fixedLines = append(fixedLines, line)

            }

        } else {

            fixedLines = append(fixedLines, line)

        }

    }



    fixed := strings.Join(fixedLines, "\n")



    // Remove docker task blocks

    dockerTaskPattern := `(?s)docker\s*\{[^}]*\}`

    re := regexp.MustCompile(dockerTaskPattern)

    fixed = re.ReplaceAllString(fixed, "// Docker config removed")



    return fixed

}



func (f *BuildFixer) fixGradleSettingsFile() error {

    settingsFile := filepath.Join(f.repoDir, "settings.gradle")

    if _, err := os.Stat(settingsFile); os.IsNotExist(err) {

        settingsFile = filepath.Join(f.repoDir, "settings.gradle.kts")

        if _, err := os.Stat(settingsFile); os.IsNotExist(err) {

            return nil

        }

    }



    content, err := os.ReadFile(settingsFile)

    if err != nil {

        return err

    }



    original := string(content)

    fixed := regexp.MustCompile(`(?s)pluginManagement\s*\{[^}]*docker[^}]*\}`).

        ReplaceAllString(original, "")



    if fixed != original {

        os.WriteFile(settingsFile+".backup", content, 0644)

        os.WriteFile(settingsFile, []byte(fixed), 0644)

    }



    return nil

}



func (f *BuildFixer) createGradleInitScript() error {

    initScript := `

allprojects {

    afterEvaluate { project ->

        // Remove docker-related tasks

        project.tasks.all { task ->

            if (task.name.toLowerCase().contains('docker')) {

                task.enabled = false

            }

        }

        

        // Remove problematic plugins

        project.plugins.removeAll { plugin ->

            def className = plugin.class.name.toLowerCase()

            className.contains('docker') || 

            className.contains('palantir') ||

            className.contains('jib')

        }

    }

}

`

    

    initPath := filepath.Join(f.repoDir, "init.gradle")

    return os.WriteFile(initPath, []byte(initScript), 0644)

}



// ==================== MAVEN FIXES ====================



func (f *BuildFixer) fixMaven() error {

    pomPath := filepath.Join(f.repoDir, "pom.xml")

    content, err := os.ReadFile(pomPath)

    if err != nil {

        return err

    }



    original := string(content)

    fixed := original



    // Remove docker plugins

    pluginPatterns := []string{

        `(?s)<plugin>.*?docker-maven-plugin.*?</plugin>`,

        `(?s)<plugin>.*?jib-maven-plugin.*?</plugin>`,

        `(?s)<plugin>.*?fabric8.*?</plugin>`,

    }



    for _, pattern := range pluginPatterns {

        re := regexp.MustCompile(pattern)

        fixed = re.ReplaceAllString(fixed, "<!-- Docker plugin removed -->")

    }



    if fixed != original {

        os.WriteFile(pomPath+".backup", content, 0644)

        os.WriteFile(pomPath, []byte(fixed), 0644)

        fmt.Println(" Fixed pom.xml - removed docker plugins")

    }



    return nil

}



// ==================== NODE.JS FIXES ====================



func (f *BuildFixer) fixNode() error {

    pkgPath := filepath.Join(f.repoDir, "package.json")

    content, err := os.ReadFile(pkgPath)

    if err != nil {

        return err

    }



    var pkg map[string]interface{}

    if err := json.Unmarshal(content, &pkg); err != nil {

        return err

    }



    modified := false



    // Fix restrictive engine requirements

    if engines, ok := pkg["engines"].(map[string]interface{}); ok {

        if node, ok := engines["node"].(string); ok {

            if strings.Contains(node, "^") || strings.Contains(node, "~") {

                engines["node"] = ">=" + strings.TrimLeft(node, "^~")

                modified = true

            }

        }

    }



    // Remove problematic scripts

    if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {

        problematicScripts := []string{"prepare", "preinstall", "postinstall"}

        for _, script := range problematicScripts {

            if _, exists := scripts[script]; exists {

                delete(scripts, script)

                modified = true

            }

        }

    }



    if modified {

        fixed, _ := json.MarshalIndent(pkg, "", "  ")

        os.WriteFile(pkgPath+".backup", content, 0644)

        os.WriteFile(pkgPath, fixed, 0644)

        fmt.Println(" Fixed package.json")

    }



    return nil

}



// ==================== PYTHON FIXES ====================



func (f *BuildFixer) fixPython() error {

    reqPath := filepath.Join(f.repoDir, "requirements.txt")

    if _, err := os.Stat(reqPath); os.IsNotExist(err) {

        return nil

    }



    content, err := os.ReadFile(reqPath)

    if err != nil {

        return err

    }



    lines := strings.Split(string(content), "\n")

    fixedLines := []string{}

    modified := false



    for _, line := range lines {

        trimmed := strings.TrimSpace(line)

        

        // Keep comments and empty lines

        if strings.HasPrefix(trimmed, "#") || trimmed == "" {

            fixedLines = append(fixedLines, line)

            continue

        }



        // Convert exact versions to minimum versions

        if strings.Contains(trimmed, "==") {

            parts := strings.Split(trimmed, "==")

            fixedLines = append(fixedLines, parts[0]+">="+parts[1])

            modified = true

        } else {

            fixedLines = append(fixedLines, line)

        }

    }



    if modified {

        fixed := strings.Join(fixedLines, "\n")

        os.WriteFile(reqPath+".backup", content, 0644)

        os.WriteFile(reqPath, []byte(fixed), 0644)

        fmt.Println(" Fixed requirements.txt")

    }



    return nil

}



// ==================== PHP FIXES ====================



func (f *BuildFixer) fixPHP() error {

    composerPath := filepath.Join(f.repoDir, "composer.json")

    if _, err := os.Stat(composerPath); os.IsNotExist(err) {

        return nil

    }



    content, err := os.ReadFile(composerPath)

    if err != nil {

        return err

    }



    var composer map[string]interface{}

    if err := json.Unmarshal(content, &composer); err != nil {

        return err

    }



    modified := false



    // Relax PHP version requirement

    if require, ok := composer["require"].(map[string]interface{}); ok {

        if php, ok := require["php"].(string); ok {

            if strings.Contains(php, "^") {

                require["php"] = ">=" + strings.TrimPrefix(php, "^")

                modified = true

            }

        }

    }



    if modified {

        fixed, _ := json.MarshalIndent(composer, "", "  ")

        os.WriteFile(composerPath+".backup", content, 0644)

        os.WriteFile(composerPath, fixed, 0644)

        fmt.Println(" Fixed composer.json")

    }



    return nil

}



// ==================== RUBY FIXES ====================



func (f *BuildFixer) fixRuby() error {

    gemfilePath := filepath.Join(f.repoDir, "Gemfile")

    if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {

        return nil

    }



    content, err := os.ReadFile(gemfilePath)

    if err != nil {

        return err

    }



    original := string(content)

    fixed := original



    // Relax Ruby version

    re := regexp.MustCompile(`ruby ['"]~>([^'"]+)['"]`)

    fixed = re.ReplaceAllString(fixed, "ruby '>=$1'")



    if fixed != original {

        os.WriteFile(gemfilePath+".backup", content, 0644)

        os.WriteFile(gemfilePath, []byte(fixed), 0644)

        fmt.Println(" Fixed Gemfile")

    }



    return nil

}
