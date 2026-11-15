package editor

import "strings"

// windowsReserved are device names Windows refuses (or maps to devices) as
// file names, regardless of extension.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidateWriteName checks a file name that is about to be created or
// overwritten in a template folder. On top of the general name rule
// ([a-zA-Z0-9._-]+) it rejects names starting with "." (hidden files, and
// the namespace of AtomicWrite's temp files) and Windows-reserved device
// names (CON, PRN, AUX, NUL, COM1-9, LPT1-9; any case, any extension).
// It returns ErrInvalidName.
func ValidateWriteName(name string) error {
	if !safeNameRe.MatchString(name) || strings.HasPrefix(name, ".") {
		return ErrInvalidName
	}
	stem, _, _ := strings.Cut(name, ".")
	if windowsReserved[strings.ToLower(stem)] {
		return ErrInvalidName
	}
	return nil
}
