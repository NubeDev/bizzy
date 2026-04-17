package models

import "time"

// NotifyPrefs stores per-user notification preferences.
// Each field lists which channels to notify on for that event type.
// An empty list means "reply to originating channel only" (the default).
type NotifyPrefs struct {
	UserID           string   `json:"user_id" gorm:"primaryKey"`
	OnWorkflowDone   []string `json:"on_workflow_done" gorm:"serializer:json"`
	OnWorkflowFailed []string `json:"on_workflow_failed" gorm:"serializer:json"`
	OnJobDone        []string `json:"on_job_done" gorm:"serializer:json"`
	OnJobFailed      []string `json:"on_job_failed" gorm:"serializer:json"`
	OnApprovalNeeded []string `json:"on_approval_needed" gorm:"serializer:json"`
	UpdatedAt        time.Time `json:"updated_at"`
}
