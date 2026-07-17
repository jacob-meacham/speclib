package source

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type Ref struct {
	Raw      string
	IsLocal  bool
	Location string
}

func Parse(source string) Ref {
	switch {
	case strings.HasPrefix(source, "file://"):
		return Ref{Raw: source, IsLocal: true, Location: strings.TrimPrefix(source, "file://")}
	case strings.HasPrefix(source, "."), strings.HasPrefix(source, "/"):
		return Ref{Raw: source, IsLocal: true, Location: source}
	default:
		return Ref{Raw: source, IsLocal: false, Location: source}
	}
}

func PickVersion(constraint string, versions []*semver.Version) (*semver.Version, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions available")
	}
	sorted := append([]*semver.Version(nil), versions...)
	sort.Sort(sort.Reverse(semver.Collection(sorted)))

	if constraint == "" || constraint == "*" {
		return sorted[0], nil
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint %q: %w", constraint, err)
	}
	for _, v := range sorted {
		if c.Check(v) {
			return v, nil
		}
	}
	return nil, fmt.Errorf("no version satisfies constraint %q", constraint)
}
