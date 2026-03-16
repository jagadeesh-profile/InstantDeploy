package runtime

import "instantdeploy/backend/pkg/models"

type Store interface {
	EnsureSchema() error
	ListDeployments() ([]models.Deployment, error)
	ListLogsByDeployment() (map[string][]models.DeploymentLog, error)
	UpsertDeployment(models.Deployment) error
	AppendLog(deploymentID string, log models.DeploymentLog) error
	DeleteDeployment(deploymentID string) error
}
