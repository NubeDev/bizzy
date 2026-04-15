package models

import "time"

// AppInstall represents a user's installation of an app, including their settings.
type AppInstall struct {
	ID          string            `json:"id"`
	AppName     string            `json:"appName"`
	AppVersion  string            `json:"appVersion"`
	WorkspaceID string            `json:"workspaceId"`
	UserID      string            `json:"userId"`
	Enabled     bool              `json:"enabled"`
	Settings    map[string]string `json:"settings"`    // non-secret settings
	Secrets     map[string]string `json:"secrets"`     // secret settings (TODO: encrypt at rest)
	Stale       bool              `json:"stale"`       // true if app version on disk differs from installed version
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func (a AppInstall) GetID() string { return a.ID }

// GetSetting returns a setting value, checking secrets first then settings.
func (a AppInstall) GetSetting(key string) string {
	if v, ok := a.Secrets[key]; ok {
		return v
	}
	return a.Settings[key]
}
