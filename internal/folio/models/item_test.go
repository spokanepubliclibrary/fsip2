package models

import (
	"encoding/json"
	"testing"
)

// --- Subject.UnmarshalJSON ---

func TestSubject_UnmarshalJSON_String(t *testing.T) {
	var s Subject
	if err := json.Unmarshal([]byte(`"History"`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "History" {
		t.Errorf("got %q, want %q", s.Value, "History")
	}
}

func TestSubject_UnmarshalJSON_Object(t *testing.T) {
	var s Subject
	if err := json.Unmarshal([]byte(`{"value":"Science"}`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "Science" {
		t.Errorf("got %q, want %q", s.Value, "Science")
	}
}

// --- Series.UnmarshalJSON ---

func TestSeries_UnmarshalJSON_String(t *testing.T) {
	var s Series
	if err := json.Unmarshal([]byte(`"My Series"`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "My Series" {
		t.Errorf("got %q, want %q", s.Value, "My Series")
	}
}

func TestSeries_UnmarshalJSON_Object(t *testing.T) {
	var s Series
	if err := json.Unmarshal([]byte(`{"value":"Another Series"}`), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Value != "Another Series" {
		t.Errorf("got %q, want %q", s.Value, "Another Series")
	}
}

// --- Item.IsAvailable ---

func TestItem_IsAvailable(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"available", "Available", true},
		{"checked out", "Checked out", false},
		{"in transit", "In transit", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &Item{Status: ItemStatus{Name: tt.status}}
			if got := item.IsAvailable(); got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Item.IsCheckedOut ---

func TestItem_IsCheckedOut(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"checked out", "Checked out", true},
		{"available", "Available", false},
		{"in transit", "In transit", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &Item{Status: ItemStatus{Name: tt.status}}
			if got := item.IsCheckedOut(); got != tt.want {
				t.Errorf("IsCheckedOut() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Item.GetEffectiveCallNumber ---

func TestItem_GetEffectiveCallNumber_WithComponents(t *testing.T) {
	item := &Item{
		EffectiveCallNumberComponents: CallNumberComponents{
			CallNumber: "QA76.9",
			Prefix:     "REF",
			Suffix:     "v.2",
		},
	}
	got := item.GetEffectiveCallNumber()
	want := "REF QA76.9 v.2"
	if got != want {
		t.Errorf("GetEffectiveCallNumber() = %q, want %q", got, want)
	}
}

func TestItem_GetEffectiveCallNumber_NumberOnly(t *testing.T) {
	item := &Item{
		EffectiveCallNumberComponents: CallNumberComponents{
			CallNumber: "QA76.9",
		},
	}
	got := item.GetEffectiveCallNumber()
	if got != "QA76.9" {
		t.Errorf("GetEffectiveCallNumber() = %q, want %q", got, "QA76.9")
	}
}

func TestItem_GetEffectiveCallNumber_PrefixOnly(t *testing.T) {
	item := &Item{
		EffectiveCallNumberComponents: CallNumberComponents{
			CallNumber: "QA76.9",
			Prefix:     "REF",
		},
	}
	got := item.GetEffectiveCallNumber()
	if got != "REF QA76.9" {
		t.Errorf("GetEffectiveCallNumber() = %q, want %q", got, "REF QA76.9")
	}
}

func TestItem_GetEffectiveCallNumber_SuffixOnly(t *testing.T) {
	item := &Item{
		EffectiveCallNumberComponents: CallNumberComponents{
			CallNumber: "QA76.9",
			Suffix:     "c.1",
		},
	}
	got := item.GetEffectiveCallNumber()
	if got != "QA76.9 c.1" {
		t.Errorf("GetEffectiveCallNumber() = %q, want %q", got, "QA76.9 c.1")
	}
}

func TestItem_GetEffectiveCallNumber_FallbackToCallNumber(t *testing.T) {
	item := &Item{
		CallNumber: "PS3563.A31",
	}
	got := item.GetEffectiveCallNumber()
	if got != "PS3563.A31" {
		t.Errorf("GetEffectiveCallNumber() = %q, want %q", got, "PS3563.A31")
	}
}

func TestItem_GetEffectiveCallNumber_Empty(t *testing.T) {
	item := &Item{}
	got := item.GetEffectiveCallNumber()
	if got != "" {
		t.Errorf("GetEffectiveCallNumber() = %q, want empty string", got)
	}
}

// --- Item.GetTitle ---

func TestItem_GetTitle_FromItem(t *testing.T) {
	item := &Item{Title: "Direct Title"}
	if got := item.GetTitle(); got != "Direct Title" {
		t.Errorf("GetTitle() = %q, want %q", got, "Direct Title")
	}
}

func TestItem_GetTitle_FromInstance(t *testing.T) {
	item := &Item{
		Instance: &Instance{Title: "Instance Title"},
	}
	if got := item.GetTitle(); got != "Instance Title" {
		t.Errorf("GetTitle() = %q, want %q", got, "Instance Title")
	}
}

func TestItem_GetTitle_ItemTitleTakesPrecedence(t *testing.T) {
	item := &Item{
		Title:    "Item Title",
		Instance: &Instance{Title: "Instance Title"},
	}
	if got := item.GetTitle(); got != "Item Title" {
		t.Errorf("GetTitle() = %q, want %q", got, "Item Title")
	}
}

func TestItem_GetTitle_Empty(t *testing.T) {
	item := &Item{}
	if got := item.GetTitle(); got != "" {
		t.Errorf("GetTitle() = %q, want empty string", got)
	}
}

// --- Item.GetCheckinNotes ---

func TestItem_GetCheckinNotes_ReturnsCheckInNotes(t *testing.T) {
	item := &Item{
		CirculationNotes: []CirculationNote{
			{NoteType: "Check in", Note: "Handle with care"},
			{NoteType: "Check out", Note: "Inspect condition"},
			{NoteType: "Check in", Note: "Missing piece"},
		},
	}
	notes := item.GetCheckinNotes()
	if len(notes) != 2 {
		t.Fatalf("GetCheckinNotes() returned %d notes, want 2", len(notes))
	}
	if notes[0] != "Handle with care" {
		t.Errorf("notes[0] = %q, want %q", notes[0], "Handle with care")
	}
	if notes[1] != "Missing piece" {
		t.Errorf("notes[1] = %q, want %q", notes[1], "Missing piece")
	}
}

func TestItem_GetCheckinNotes_NoCheckInNotes(t *testing.T) {
	item := &Item{
		CirculationNotes: []CirculationNote{
			{NoteType: "Check out", Note: "Some note"},
		},
	}
	notes := item.GetCheckinNotes()
	if notes != nil {
		t.Errorf("GetCheckinNotes() = %v, want nil", notes)
	}
}

func TestItem_GetCheckinNotes_Empty(t *testing.T) {
	item := &Item{}
	notes := item.GetCheckinNotes()
	if notes != nil {
		t.Errorf("GetCheckinNotes() = %v, want nil", notes)
	}
}
