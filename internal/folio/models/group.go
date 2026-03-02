package models

// PatronGroup represents a FOLIO patron group (user group)
type PatronGroup struct {
	ID                     string   `json:"id"`
	Group                  string   `json:"group"` // Group name
	Desc                   string   `json:"desc"`  // Group description
	ExpirationOffsetInDays int      `json:"expirationOffsetInDays,omitempty"`
	Metadata               Metadata `json:"metadata,omitempty"`
}

// PatronGroupCollection represents a collection of patron groups
type PatronGroupCollection struct {
	UserGroups   []PatronGroup `json:"usergroups"`
	TotalRecords int           `json:"totalRecords"`
}
