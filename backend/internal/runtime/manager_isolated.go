package runtime



import (

    "context"

    "fmt"

    "os"

    "path/filepath"

)



// cloneRepository - UPDATED for Linux container paths

func (m *EnhancedManager) cloneRepository(ctx context.Context, repo, branch, deploymentID string) (string, error) {

    // Use Linux tmp directory (inside container)

    tempDir := filepath.Join("/tmp/builds", deploymentID)

    

    // Create directory

    if err := os.MkdirAll(tempDir, 0755); err != nil {

        return "", fmt.Errorf("failed to create build dir: %w", err)

    }



    database.AddDeploymentLog(deploymentID, "info", 

        fmt.Sprintf("Cloning repository to %s", tempDir), "build")



    // Git clone (runs inside Linux container)

    cmd := exec.CommandContext(ctx, "git", "clone", 

        "-b", branch, 

        "--depth", "1", 

        repo, 

        tempDir)

    

    output, err := cmd.CombinedOutput()

    if err != nil {

        database.AddDeploymentLog(deploymentID, "error", string(output), "build")

        os.RemoveAll(tempDir)

        return "", fmt.Errorf("git clone failed: %w", err)

    }



    database.AddDeploymentLog(deploymentID, "info", 

        "Repository cloned successfully", "build")

    

    return tempDir, nil

}



// buildImage - UPDATED to ensure Linux paths in build context

func (m *EnhancedManager) buildImage(ctx context.Context, buildContext, dockerfile, deploymentID string) (string, error) {

    database.AddDeploymentLog(deploymentID, "info", 

        fmt.Sprintf("Building image from context: %s", buildContext), "build")



    // All paths are now Linux paths

    // buildContext = /tmp/builds/deploy-abc123

    // No C:// paths!



    // Create tar archive

    tarBuf, err := createTarArchive(buildContext)

    if err != nil {

        return "", fmt.Errorf("failed to create tar: %w", err)

    }



    // Build options (same as before)

    buildOptions := types.ImageBuildOptions{

        Dockerfile: "Dockerfile.generated",

        Tags:       []string{fmt.Sprintf("instantdeploy-%s:latest", deploymentID)},

        Remove:     true,

        Memory:     1024 * 1024 * 1024, // 1GB

        MemorySwap: 1024 * 1024 * 1024,

        CPUQuota:   200000, // 2 CPUs

        Version:    types.BuilderBuildKit,

    }



    // Build image

    response, err := m.docker.ImageBuild(ctx, tarBuf, buildOptions)

    if err != nil {

        return "", fmt.Errorf("image build failed: %w", err)

    }

    defer response.Body.Close()



    // Stream logs

    scanner := bufio.NewScanner(response.Body)

    for scanner.Scan() {

        logLine := scanner.Text()

        database.AddDeploymentLog(deploymentID, "info", logLine, "build")

    }



    if err := scanner.Err(); err != nil {

        return "", err

    }



    // Get image ID

    images, err := m.docker.ImageList(ctx, types.ImageListOptions{})

    if err != nil {

        return "", err

    }



    for _, img := range images {

        for _, tag := range img.RepoTags {

            if strings.Contains(tag, deploymentID) {

                database.AddDeploymentLog(deploymentID, "info", 

                    "Image built successfully", "build")

                return img.ID, nil

            }

        }

    }



    return "", fmt.Errorf("image not found after build")

}





================================================================================

PART 5: STARTUP SCRIPT (ONE COMMAND TO RULE THEM ALL)
