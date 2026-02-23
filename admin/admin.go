// Package admin provides admin panel resource definitions.
package admin

// Resource describes a model registered with the admin panel.
type Resource struct {
	// Name is the display name for this resource.
	Name string

	// Model is the URL-safe identifier (e.g., "user", "article").
	Model string

	// Fields lists the field names to display in the admin list view.
	Fields []string
}
