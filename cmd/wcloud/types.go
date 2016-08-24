package main

import (
	"time"
)

// Deployment describes a deployment
type Deployment struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ImageName string    `json:"image_name"`
	Version   string    `json:"version"`
	Priority  int       `json:"priority"`
	State     string    `json:"status"`

	TriggeringUser string `json:"triggering_user"`
}

// Config for the deployment system for a user.
type Config struct {
	RepoURL        string `json:"repo_url" yaml:"repo_url"`
	RepoPath       string `json:"repo_path" yaml:"repo_path"`
	RepoKey        string `json:"repo_key" yaml:"repo_key"`
	KubeconfigPath string `json:"kubeconfig_path" yaml:"kubeconfig_path"`

	Notifications []NotificationConfig `json:"notifications" yaml:"notifications"`

	// Globs of files not to change, relative to the route of the repo
	ConfigFileBlackList []string `json:"config_file_black_list" yaml:"config_file_black_list"`

	CommitMessageTemplate string `json:"commit_message_template" yaml:"commit_message_template"` // See https://golang.org/pkg/text/template/
}

// NotificationConfig describes how to send notifications
type NotificationConfig struct {
	SlackWebhookURL string `json:"slack_webhook_url" yaml:"slack_webhook_url"`
	SlackUsername   string `json:"slack_username" yaml:"slack_username"`
	MessageTemplate string `json:"message_template" yaml:"message_template"`
}

// InstancesFile is used to store local wcloud instance configs
type InstancesFile struct {
	Instances map[string]*Instance `yaml:"instances"`
	Current   string               `yaml:"current_instance"`
}

// Available lists the names of available instances in this file.
func (i *InstancesFile) Available() []string {
	var a []string
	for name := range i.Instances {
		a = append(a, name)
	}
	return a
}

// Instance is used to store local wcloud instance configs
type Instance struct {
	ServiceToken string `yaml:"service_token"`
	BaseURL      string `yaml:"base_url"`
}
