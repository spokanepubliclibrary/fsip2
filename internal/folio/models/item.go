package models

import (
	"encoding/json"
	"time"
)

// Item represents a FOLIO item
type Item struct {
	ID                                 string               `json:"id"`
	HoldingsRecordID                   string               `json:"holdingsRecordId"`
	InstanceID                         string               `json:"instanceId,omitempty"`
	FormerIds                          []string             `json:"formerIds,omitempty"`
	DiscoverySuppress                  bool                 `json:"discoverySuppress,omitempty"`
	Title                              string               `json:"title,omitempty"`
	ContributorNames                   []ContributorName    `json:"contributorNames,omitempty"`
	CallNumber                         string               `json:"callNumber,omitempty"`
	Barcode                            string               `json:"barcode"`
	EffectiveShelvingOrder             string               `json:"effectiveShelvingOrder,omitempty"`
	AccessionNumber                    string               `json:"accessionNumber,omitempty"`
	ItemLevelCallNumber                string               `json:"itemLevelCallNumber,omitempty"`
	ItemLevelCallNumberPrefix          string               `json:"itemLevelCallNumberPrefix,omitempty"`
	ItemLevelCallNumberSuffix          string               `json:"itemLevelCallNumberSuffix,omitempty"`
	ItemLevelCallNumberTypeID          string               `json:"itemLevelCallNumberTypeId,omitempty"`
	EffectiveCallNumberComponents      CallNumberComponents `json:"effectiveCallNumberComponents,omitempty"`
	Volume                             string               `json:"volume,omitempty"`
	Enumeration                        string               `json:"enumeration,omitempty"`
	Chronology                         string               `json:"chronology,omitempty"`
	YearCaption                        []string             `json:"yearCaption,omitempty"`
	ItemIdentifier                     string               `json:"itemIdentifier,omitempty"`
	CopyNumber                         string               `json:"copyNumber,omitempty"`
	NumberOfPieces                     string               `json:"numberOfPieces,omitempty"`
	DescriptionOfPieces                string               `json:"descriptionOfPieces,omitempty"`
	NumberOfMissingPieces              string               `json:"numberOfMissingPieces,omitempty"`
	MissingPieces                      string               `json:"missingPieces,omitempty"`
	MissingPiecesDate                  string               `json:"missingPiecesDate,omitempty"`
	ItemDamagedStatusID                string               `json:"itemDamagedStatusId,omitempty"`
	ItemDamagedStatusDate              string               `json:"itemDamagedStatusDate,omitempty"`
	Status                             ItemStatus           `json:"status"`
	MaterialTypeID                     string               `json:"materialTypeId"`
	PermanentLoanTypeID                string               `json:"permanentLoanTypeId"`
	TemporaryLoanTypeID                string               `json:"temporaryLoanTypeId,omitempty"`
	PermanentLocationID                string               `json:"permanentLocationId"`
	TemporaryLocationID                string               `json:"temporaryLocationId,omitempty"`
	EffectiveLocationID                string               `json:"effectiveLocationId"`
	InTransitDestinationServicePointID string               `json:"inTransitDestinationServicePointId,omitempty"`
	StatisticalCodeIds                 []string             `json:"statisticalCodeIds,omitempty"`
	PurchaseOrderLineIdentifier        string               `json:"purchaseOrderLineIdentifier,omitempty"`
	Tags                               Tags                 `json:"tags,omitempty"`
	LastCheckIn                        *LastCheckIn         `json:"lastCheckIn,omitempty"`
	CirculationNotes                   []CirculationNote    `json:"circulationNotes,omitempty"`
	Metadata                           Metadata             `json:"metadata,omitempty"`
	// Populated fields
	Instance     *Instance     `json:"instance,omitempty"`
	Holdings     *Holdings     `json:"holdings,omitempty"`
	Location     *Location     `json:"effectiveLocation,omitempty"`
	MaterialType *MaterialType `json:"materialType,omitempty"`
}

// ContributorName represents a contributor
type ContributorName struct {
	Name string `json:"name"`
}

// CallNumberComponents represents call number components
type CallNumberComponents struct {
	CallNumber string `json:"callNumber,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	Suffix     string `json:"suffix,omitempty"`
}

// ItemStatus represents the status of an item
type ItemStatus struct {
	Name string     `json:"name"` // Available, Checked out, In transit, etc.
	Date *time.Time `json:"date,omitempty"`
}

// Tags represents tags on an item
type Tags struct {
	TagList []string `json:"tagList,omitempty"`
}

// LastCheckIn represents last check-in information
type LastCheckIn struct {
	DateTime       *time.Time `json:"dateTime,omitempty"`
	ServicePointID string     `json:"servicePointId,omitempty"`
	StaffMemberID  string     `json:"staffMemberId,omitempty"`
}

// ItemCollection represents a collection of items
type ItemCollection struct {
	Items        []Item `json:"items"`
	TotalRecords int    `json:"totalRecords"`
}

// Instance represents a bibliographic instance
type Instance struct {
	ID                   string             `json:"id"`
	HrID                 string             `json:"hrid,omitempty"`
	Source               string             `json:"source,omitempty"`
	Title                string             `json:"title"`
	IndexTitle           string             `json:"indexTitle,omitempty"`
	AlternativeTitles    []AlternativeTitle `json:"alternativeTitles,omitempty"`
	Editions             []string           `json:"editions,omitempty"`
	Series               []Series           `json:"series,omitempty"`
	Identifiers          []Identifier       `json:"identifiers,omitempty"`
	Contributors         []Contributor      `json:"contributors,omitempty"`
	Subjects             []Subject          `json:"subjects,omitempty"`
	Classifications      []Classification   `json:"classifications,omitempty"`
	Publication          []Publication      `json:"publication,omitempty"`
	PublicationFrequency []string           `json:"publicationFrequency,omitempty"`
	PublicationRange     []string           `json:"publicationRange,omitempty"`
	ElectronicAccess     []ElectronicAccess `json:"electronicAccess,omitempty"`
	InstanceTypeID       string             `json:"instanceTypeId"`
	InstanceFormatIds    []string           `json:"instanceFormatIds,omitempty"`
	PhysicalDescriptions []string           `json:"physicalDescriptions,omitempty"`
	Languages            []string           `json:"languages,omitempty"`
	Notes                []Note             `json:"notes,omitempty"`
	Metadata             Metadata           `json:"metadata,omitempty"`
}

// AlternativeTitle represents an alternative title
type AlternativeTitle struct {
	AlternativeTitleTypeID string `json:"alternativeTitleTypeId,omitempty"`
	AlternativeTitle       string `json:"alternativeTitle"`
}

// Identifier represents an identifier (ISBN, ISSN, etc.)
type Identifier struct {
	Value            string `json:"value"`
	IdentifierTypeID string `json:"identifierTypeId"`
}

// Contributor represents a contributor to an instance
type Contributor struct {
	Name                  string `json:"name"`
	ContributorTypeID     string `json:"contributorTypeId,omitempty"`
	ContributorTypeText   string `json:"contributorTypeText,omitempty"`
	ContributorNameTypeID string `json:"contributorNameTypeId,omitempty"`
	Primary               bool   `json:"primary,omitempty"`
}

// Subject represents a subject heading (can be string or object)
type Subject struct {
	Value string `json:"value,omitempty"` // For object format
}

// UnmarshalJSON implements custom unmarshaling for Subject to handle both string and object formats
func (s *Subject) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.Value = str
		return nil
	}

	// If that fails, try as object
	type SubjectAlias Subject
	var obj SubjectAlias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*s = Subject(obj)
	return nil
}

// Series represents a series (can be string or object)
type Series struct {
	Value string `json:"value,omitempty"` // For object format
}

// UnmarshalJSON implements custom unmarshaling for Series to handle both string and object formats
func (s *Series) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.Value = str
		return nil
	}

	// If that fails, try as object
	type SeriesAlias Series
	var obj SeriesAlias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*s = Series(obj)
	return nil
}

// Classification represents a classification
type Classification struct {
	ClassificationNumber string `json:"classificationNumber"`
	ClassificationTypeID string `json:"classificationTypeId"`
}

// Publication represents publication information
type Publication struct {
	Publisher         string `json:"publisher,omitempty"`
	Place             string `json:"place,omitempty"`
	DateOfPublication string `json:"dateOfPublication,omitempty"`
	Role              string `json:"role,omitempty"`
}

// ElectronicAccess represents electronic access information
type ElectronicAccess struct {
	URI                    string `json:"uri,omitempty"`
	LinkText               string `json:"linkText,omitempty"`
	MaterialsSpecification string `json:"materialsSpecification,omitempty"`
	PublicNote             string `json:"publicNote,omitempty"`
	RelationshipID         string `json:"relationshipId,omitempty"`
}

// Note represents a note
type Note struct {
	NoteTypeID string `json:"instanceNoteTypeId,omitempty"`
	Note       string `json:"note"`
	StaffOnly  bool   `json:"staffOnly,omitempty"`
}

// Holdings represents holdings information
type Holdings struct {
	ID                  string   `json:"id"`
	HrID                string   `json:"hrid,omitempty"`
	InstanceID          string   `json:"instanceId"`
	PermanentLocationID string   `json:"permanentLocationId"`
	TemporaryLocationID string   `json:"temporaryLocationId,omitempty"`
	EffectiveLocationID string   `json:"effectiveLocationId"`
	CallNumber          string   `json:"callNumber,omitempty"`
	Metadata            Metadata `json:"metadata,omitempty"`
}

// Location represents a location
type Location struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Code                 string   `json:"code"`
	Description          string   `json:"description,omitempty"`
	DiscoveryDisplayName string   `json:"discoveryDisplayName,omitempty"`
	IsActive             bool     `json:"isActive"`
	InstitutionID        string   `json:"institutionId"`
	CampusID             string   `json:"campusId"`
	LibraryID            string   `json:"libraryId"`
	PrimaryServicePoint  string   `json:"primaryServicePoint,omitempty"`
	ServicePointIds      []string `json:"servicePointIds,omitempty"`
	Metadata             Metadata `json:"metadata,omitempty"`
}

// MaterialType represents a material type
type MaterialType struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source,omitempty"`
}

// ServicePoint represents a service point
type ServicePoint struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Code                 string   `json:"code"`
	DiscoveryDisplayName string   `json:"discoveryDisplayName,omitempty"`
	Description          string   `json:"description,omitempty"`
	ShelvingLagTime      int      `json:"shelvingLagTime,omitempty"`
	PickupLocation       bool     `json:"pickupLocation"`
	Metadata             Metadata `json:"metadata,omitempty"`
}

// IsAvailable checks if the item is available for checkout
func (i *Item) IsAvailable() bool {
	return i.Status.Name == "Available"
}

// IsCheckedOut checks if the item is checked out
func (i *Item) IsCheckedOut() bool {
	return i.Status.Name == "Checked out"
}

// GetEffectiveCallNumber returns the effective call number for the item
func (i *Item) GetEffectiveCallNumber() string {
	if i.EffectiveCallNumberComponents.CallNumber != "" {
		prefix := i.EffectiveCallNumberComponents.Prefix
		suffix := i.EffectiveCallNumberComponents.Suffix
		callNumber := i.EffectiveCallNumberComponents.CallNumber

		if prefix != "" {
			callNumber = prefix + " " + callNumber
		}
		if suffix != "" {
			callNumber = callNumber + " " + suffix
		}
		return callNumber
	}
	return i.CallNumber
}

// GetTitle returns the item's title (from item or instance)
func (i *Item) GetTitle() string {
	if i.Title != "" {
		return i.Title
	}
	if i.Instance != nil {
		return i.Instance.Title
	}
	return ""
}

// CirculationNote represents a circulation note on an item
type CirculationNote struct {
	ID        string      `json:"id"`
	NoteType  string      `json:"noteType"` // e.g., "Check in", "Check out"
	Note      string      `json:"note"`
	Source    *NoteSource `json:"source,omitempty"`
	Date      string      `json:"date,omitempty"`
	StaffOnly bool        `json:"staffOnly,omitempty"`
}

// NoteSource represents the staff member who created a note
type NoteSource struct {
	ID       string        `json:"id"`
	Personal *PersonalInfo `json:"personal,omitempty"`
}

// GetCheckinNotes returns all "Check in" circulation notes
func (i *Item) GetCheckinNotes() []string {
	var notes []string
	for _, note := range i.CirculationNotes {
		if note.NoteType == "Check in" {
			notes = append(notes, note.Note)
		}
	}
	return notes
}
