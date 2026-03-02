package mediatype

import "testing"

func TestMapToSIP2MediaType(t *testing.T) {
	tests := []struct {
		name         string
		materialType string
		want         string
	}{
		// Book variants
		{"plain book", "Book", Book},
		{"book lowercase", "book", Book},
		{"book with diskette", "Book with Diskette", BookWithDiskette},
		{"book with floppy", "Book with Floppy", BookWithDiskette},
		{"book with cd", "Book with CD", BookWithCD},
		{"book with cd-rom", "Book with CD-ROM", BookWithCD},
		{"book with audio", "Book with Audio Tape", BookWithAudioTape},
		{"book with cassette", "Book with Cassette", BookWithAudioTape},

		// Magazine/periodical
		{"magazine", "Magazine", Magazine},
		{"periodical", "Periodical", Magazine},

		// Bound journal
		{"bound journal", "Bound Journal", BoundJournal},
		{"serial journal", "Journal Serial", BoundJournal},

		// Audio types
		{"audio tape", "Audio Tape", AudioTape},
		{"audio cassette", "Audio Cassette", AudioTape},
		{"audio cd (no tape)", "Audio Recording", CDOrCDROM},

		// Video types
		{"video tape", "Video Tape", VideoTape},
		{"vhs", "VHS Video", VideoTape},
		{"video cassette", "Video Cassette", VideoTape},
		{"dvd", "DVD", CDOrCDROM},

		// CD/disc types
		{"cd", "CD", CDOrCDROM},
		{"dvd explicit", "DVD", CDOrCDROM},
		{"disc", "Optical Disc", CDOrCDROM},
		{"blu-ray", "Blu-ray Disc", CDOrCDROM},

		// Diskette/floppy standalone
		{"diskette", "Diskette", Diskette},
		{"floppy", "Floppy Disk", Diskette},

		// Unknown types → Other
		{"unknown", "Microfilm", Other},
		{"empty string", "", Other},
		{"map", "Map", Other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapToSIP2MediaType(tt.materialType)
			if got != tt.want {
				t.Errorf("MapToSIP2MediaType(%q) = %q, want %q", tt.materialType, got, tt.want)
			}
		})
	}
}
