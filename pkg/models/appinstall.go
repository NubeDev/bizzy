package models

import "time"

// AppInstall represents a user's installation of an app, including their settings.
type AppInstall struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	AppName     string            `json:"appName" gorm:"index"`
	AppVersion  string            `json:"appVersion"`
	WorkspaceID string            `json:"workspaceId"`
	UserID      string            `json:"userId" gorm:"index"`
	Enabled     bool              `json:"enabled"`
	Settings    map[string]string `json:"settings" gorm:"serializer:json"`    // non-secret settings
	Secrets     map[string]string `json:"secrets" gorm:"serializer:json"`     // secret settings (TODO: encrypt at rest)
	Stale       bool              `json:"stale"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// GetSetting returns a setting value, checking secrets first then settings.
func (a AppInstall) GetSetting(key string) string {
	if v, ok := a.Secrets[key]; ok {
		return v
	}
	return a.Settings[key]
}
