package models

import "time"

// --- Store App ---

type Visibility string

const (
	VisibilityPrivate  Visibility = "private"
	VisibilityShared   Visibility = "shared"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPublic   Visibility = "public"
)

// StoreApp represents a user-created app stored in the database.
type StoreApp struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name" gorm:"uniqueIndex"`
	DisplayName string     `json:"displayName"`
	Description string     `json:"description"`
	LongDesc    string     `json:"longDescription"`
	Version     string     `json:"version"`
	Icon        string     `json:"icon"`
	Color       string     `json:"color"`
	Category    string     `json:"category" gorm:"index"`
	Tags        []string   `json:"tags" gorm:"serializer:json"`

	// Ownership.
	AuthorID    string     `json:"authorId" gorm:"index"`
	AuthorName  string     `json:"authorName"`
	WorkspaceID string     `json:"workspaceId"`

	// Visibility.
	Visibility Visibility  `json:"visibility"`

	// Content.
	Permissions Permissions  `json:"permissions" gorm:"serializer:json"`
	Settings    []SettingDef `json:"settings" gorm:"serializer:json"`
	Tools       []StoreTool  `json:"tools" gorm:"serializer:json"`
	Prompts     []StorePrompt `json:"prompts" gorm:"serializer:json"`

	// Stats.
	InstallCount   int     `json:"installCount"`
	ActiveInstalls int     `json:"activeInstalls"`
	AvgRating      float64 `json:"avgRating"`
	ReviewCount    int     `json:"reviewCount"`

	// Timestamps.
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

// Permissions declares what an app is allowed to do (reused from apps.App).
type Permissions struct {
	AllowedHosts     []string `json:"allowedHosts"`
	DefaultToolClass string   `json:"defaultToolClass"`
	Secrets          []string `json:"secrets"`
}

// SettingDef describes a user-configurable setting (reused from apps.App).
type SettingDef struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Default  string `json:"default"`
}

// StoreTool is an inline tool definition stored in the database.
type StoreTool struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	ToolClass   string                `json:"toolClass"`
	Mode        string                `json:"mode,omitempty"`
	Params      map[string]ToolParam  `json:"params"`
	Script      string                `json:"script"`
}

// ToolParam describes a single tool parameter.
type ToolParam struct {
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// StorePrompt is an inline prompt definition stored in the database.
type StorePrompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Body        string           `json:"body"`
}

// PromptArgument describes a single prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// --- Reviews ---

type AppReview struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	AppID     string    `json:"appId" gorm:"index"`
	UserID    string    `json:"userId" gorm:"index"`
	UserName  string    `json:"userName"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- App Shares ---

// AppShare represents a share invite for a shared-visibility store app.
type AppShare struct {
	ID        string     `json:"id" gorm:"primaryKey"`
	AppID     string     `json:"appId" gorm:"index"`
	InvitedBy string     `json:"invitedBy"`
	InviteeID string     `json:"inviteeId,omitempty"`
	Token     string     `json:"token,omitempty" gorm:"index"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// --- Categories ---

var Categories = []string{
	"iot-devices",
	"analytics",
	"devops",
	"marketing",
	"design",
	"utilities",
	"integrations",
	"automation",
}

func ValidCategory(c string) bool {
	for _, cat := range Categories {
		if cat == c {
			return true
		}
	}
	return false
}
