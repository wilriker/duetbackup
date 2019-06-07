package duetbackup

import "strings"

type Excludes struct {
	excls []string
}

func (e *Excludes) String() string {
	return strings.Join(e.excls, ",")
}

func (e *Excludes) Set(value string) error {
	e.excls = append(e.excls, CleanPath(value))
	return nil
}

// Contains checks if the given path starts with any of the known excludes
func (e *Excludes) Contains(path string) bool {
	for _, excl := range e.excls {
		if strings.HasPrefix(path, excl) {
			return true
		}
	}
	return false
}
