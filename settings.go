package gonf

import (
	"fmt"
	"strings"
)

type setting struct {
	Name        string
	Description string
	Env         string
	Options     []string
}

// Check for a matching option, and whether that option is greedy.
func (s *setting) Match(exists string) (bool, bool) {
	for _, o := range s.Options {
		if o == exists {
			return true, false
		} else if o == exists+":" {
			return true, true
		}
	}
	return false, false
}

// Format the combined environment and command line options for a setting.
func (s setting) String() string {
	o := strings.Replace(strings.Join(s.Options, ", "), ":", "", -1)
	if o == "" {
		o = s.Env
	} else if s.Env != "" {
		o += " (" + s.Env + ")"
	}
	return fmt.Sprintf("\t%-30s\n\t\t%s", o, s.Description)
}
