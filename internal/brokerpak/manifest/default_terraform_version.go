package manifest

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

func (m *Manifest) DefaultTerraformVersion() (*version.Version, error) {
	versions, err := m.TerraformVersions()
	if err != nil {
		return nil, err
	}

	for _, r := range versions {
		if r.Default {
			return r.Version, nil
		}
	}

	switch len(versions) {
	case 0:
		return &version.Version{}, fmt.Errorf("terraform not found")
	case 1:
		return versions[0].Version, nil
	default:
		return &version.Version{}, fmt.Errorf("no default terraform found")
	}
}
