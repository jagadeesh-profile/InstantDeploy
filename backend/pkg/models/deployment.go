package models

import "time"

type Deployment struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId,omitempty"`
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Status     string    `json:"status"`
	URL        string    `json:"url"`
	LocalURL   string    `json:"localUrl,omitempty"`
	RepoURL    string    `json:"repoUrl,omitempty"`
	Image      string    `json:"image,omitempty"`
	Container  string    `json:"container,omitempty"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type DeploymentLog struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type Repository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}
