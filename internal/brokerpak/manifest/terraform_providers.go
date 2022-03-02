package manifest

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry/cloud-service-broker/internal/tfproviderfqn"
	"github.com/hashicorp/go-version"
)

type TerraformProvider struct {
	// Name holds the name of this resource. e.g. terraform-provider-google-beta
	Name string `yaml:"name"`

	// Version holds the version of the resource e.g. 1.19.0
	Version *version.Version

	// Provider is the fully qualified name
	Provider tfproviderfqn.TfProviderFQN
}

func (m *Manifest) TerraformProviders() (result []TerraformProvider, err error) {
	for _, r := range m.TerraformResources {
		if strings.HasPrefix(r.Name, "terraform-provider-") {
			v, err := version.NewVersion(r.Version)
			if err != nil {
				return nil, fmt.Errorf("error parsing version for terraform provider %q: %w", r.Name, err)
			}

			fqn, err := tfproviderfqn.New(r.Name, r.Provider)
			if err != nil {
				return nil, fmt.Errorf("error parsing terraform provider fqn for %q: %w", r.Name, err)
			}

			result = append(result, TerraformProvider{
				Name:     r.Name,
				Version:  v,
				Provider: fqn,
			})
		}
	}
	return result, nil
}
