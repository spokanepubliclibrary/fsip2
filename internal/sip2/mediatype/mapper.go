package mediatype

import "strings"

// SIP2 Media Type Codes
const (
	Other             = "000"
	Book              = "001"
	Magazine          = "002"
	BoundJournal      = "003"
	AudioTape         = "004"
	VideoTape         = "005"
	CDOrCDROM         = "006"
	Diskette          = "007"
	BookWithDiskette  = "008"
	BookWithCD        = "009"
	BookWithAudioTape = "010"
)

// MapToSIP2MediaType maps a FOLIO material type name to a SIP2 media type code
func MapToSIP2MediaType(materialTypeName string) string {
	// Normalize to lowercase for case-insensitive matching
	name := strings.ToLower(materialTypeName)

	// Book types
	if strings.Contains(name, "book") {
		if strings.Contains(name, "diskette") || strings.Contains(name, "floppy") {
			return BookWithDiskette
		}
		if strings.Contains(name, "cd") || strings.Contains(name, "cd-rom") {
			return BookWithCD
		}
		if strings.Contains(name, "audio") || strings.Contains(name, "cassette") {
			return BookWithAudioTape
		}
		return Book
	}

	// Magazine/periodical types
	if strings.Contains(name, "magazine") || strings.Contains(name, "periodical") {
		return Magazine
	}

	// Bound journal
	if strings.Contains(name, "journal") && (strings.Contains(name, "bound") || strings.Contains(name, "serial")) {
		return BoundJournal
	}

	// Audio types
	if strings.Contains(name, "audio") {
		if strings.Contains(name, "tape") || strings.Contains(name, "cassette") {
			return AudioTape
		}
		return CDOrCDROM // Audio CDs
	}

	// Video types
	if strings.Contains(name, "video") {
		if strings.Contains(name, "tape") || strings.Contains(name, "vhs") || strings.Contains(name, "cassette") {
			return VideoTape
		}
		return CDOrCDROM // DVDs/Blu-rays
	}

	// CD/DVD/Disc types
	if strings.Contains(name, "cd") || strings.Contains(name, "dvd") ||
		strings.Contains(name, "disc") || strings.Contains(name, "blu-ray") {
		return CDOrCDROM
	}

	// Diskette/floppy
	if strings.Contains(name, "diskette") || strings.Contains(name, "floppy") {
		return Diskette
	}

	// Default to OTHER for unrecognized types
	return Other
}
