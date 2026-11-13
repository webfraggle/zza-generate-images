package renderer

import (
	"fmt"
	"regexp"
	"sync"
	"time"
	_ "time/tzdata" // embed the zone database: Windows and Alpine ship none
)

const (
	defaultTimezone = "Europe/Berlin"
	maxTimezoneLen  = 64
)

var timezoneRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+)*$`)

var berlin = sync.OnceValue(func() *time.Location {
	l, err := time.LoadLocation(defaultTimezone)
	if err != nil {
		return time.UTC
	}
	return l
})

// loadTimezone validates and loads an IANA zone name. "" means Europe/Berlin.
func loadTimezone(name string) (*time.Location, error) {
	if name == "" {
		return berlin(), nil
	}
	if len(name) > maxTimezoneLen {
		return nil, fmt.Errorf("time zone name longer than %d characters", maxTimezoneLen)
	}
	if !timezoneRe.MatchString(name) {
		return nil, fmt.Errorf("malformed time zone name")
	}
	if name == "Local" {
		return nil, fmt.Errorf("time zone %q is not allowed", name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	return loc, nil
}

// location returns the template's time zone (Berlin if unset or invalid).
func (t *Template) location() *time.Location {
	if t.loc != nil {
		return t.loc
	}
	if loc, err := loadTimezone(t.Meta.Timezone); err == nil {
		return loc
	}
	return berlin()
}
