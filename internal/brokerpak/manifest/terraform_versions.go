package manifest

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

type TerraformVersion struct {
	// Version holds the version of the resource e.g. 1.19.0
	Version *version.Version

	// Default is used to mark the default Terraform version when there is more than one
	Default bool
}

func (m *Manifest) TerraformVersions() (result []TerraformVersion, err error) {
	for _, r := range m.TerraformResources {
		if r.Name == "terraform" {
			v, err := version.NewVersion(r.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to parse terraform version: %w", err)
			}
			result = append(result, TerraformVersion{
				Version: v,
				Default: r.Default,
			})
		}
	}

	return result, nil
}
