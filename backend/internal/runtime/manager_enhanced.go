// Add this to the existing manager_enhanced.go file

// Replace the detectProjectType and generateDockerfile functions



func (m *EnhancedManager) detectProjectType(repoDir, deploymentID string) (string, map[string]interface{}, error) {

    // Use smart detector

    detector := NewSmartDetector(repoDir)

    config, err := detector.Detect()

    if err != nil {

        return "", nil, fmt.Errorf("detection failed: %w", err)

    }



    database.AddDeploymentLog(

        deploymentID,

        "info",

        fmt.Sprintf("Detected: %s (%s %s)", config.Type, config.Language, config.Framework),

        "build",

    )



    if len(config.SkipPlugins) > 0 {

        database.AddDeploymentLog(

            deploymentID,

            "warn",

            fmt.Sprintf("Found problematic plugins: %v", config.SkipPlugins),

            "build",

        )

    }



    // Apply fixes if needed

    if config.FixRequired || len(config.SkipPlugins) > 0 {

        database.AddDeploymentLog(deploymentID, "info", "Applying build fixes...", "build")

        

        fixer := NewBuildFixer(repoDir, config)

        if err := fixer.Fix(); err != nil {

            database.AddDeploymentLog(

                deploymentID,

                "warn",

                fmt.Sprintf("Fix warning: %v", err),

                "build",

            )

        } else {

            database.AddDeploymentLog(deploymentID, "info", " Build files fixed", "build")

        }

    }



    // Convert to map for compatibility

    configMap := map[string]interface{}{

        "type":         config.Type,

        "language":     config.Language,

        "framework":    config.Framework,

        "buildTool":    config.BuildTool,

        "version":      config.Version,

        "port":         config.Port,

        "buildCommand": config.BuildCommand,

        "startCommand": config.StartCommand,

        "skipPlugins":  config.SkipPlugins,

    }



    return config.Type, configMap, nil

}



func (m *EnhancedManager) generateDockerfile(repoDir, projectType string, config map[string]interface{}, deploymentID string) (string, error) {

    // Convert map to ProjectConfig

    projectConfig := &ProjectConfig{

        Type:         projectType,

        Language:     getStringFromMap(config, "language"),

        Framework:    getStringFromMap(config, "framework"),

        BuildTool:    getStringFromMap(config, "buildTool"),

        Port:         getIntFromMap(config, "port"),

        BuildCommand: getStringFromMap(config, "buildCommand"),

        StartCommand: getStringFromMap(config, "startCommand"),

        Version:      make(map[string]string),

    }



    if versions, ok := config["version"].(map[string]string); ok {

        projectConfig.Version = versions

    }



    // Generate Dockerfile

    dockerfile, err := GenerateDockerfile(projectConfig)

    if err != nil {

        return "", err

    }



    // Write to file

    dockerfilePath := filepath.Join(repoDir, "Dockerfile.instantdeploy")

    if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {

        return "", err

    }



    database.AddDeploymentLog(deploymentID, "info", " Generated optimized Dockerfile", "build")

    return dockerfile, nil

}



// Helper functions

func getStringFromMap(m map[string]interface{}, key string) string {

    if v, ok := m[key].(string); ok {

        return v

    }

    return ""

}



func getIntFromMap(m map[string]interface{}, key string) int {

    if v, ok := m[key].(int); ok {

        return v

    }

    if v, ok := m[key].(float64); ok {

        return int(v)

    }

    return 0

}
