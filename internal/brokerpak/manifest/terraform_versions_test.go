package manifest_test

import (
	"github.com/cloudfoundry/cloud-service-broker/internal/brokerpak/manifest"
	"github.com/hashicorp/go-version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TerraformVersions", func() {
	It("returns a slice of Terraform versions", func() {
		m, err := manifest.Parse(fakeManifest(
			withAdditionalEntry("terraform_binaries", map[string]interface{}{
				"name":    "terraform",
				"version": "1.1.5",
				"default": false,
			}),
			withAdditionalEntry("terraform_binaries", map[string]interface{}{
				"name":    "terraform",
				"version": "1.1.6",
				"default": true,
			}),
		))
		Expect(err).NotTo(HaveOccurred())

		Expect(m.TerraformVersions()).To(Equal([]manifest.TerraformVersion{
			{
				Version: version.Must(version.NewVersion("1.1.4")),
				Default: false,
			},
			{
				Version: version.Must(version.NewVersion("1.1.5")),
				Default: false,
			},
			{
				Version: version.Must(version.NewVersion("1.1.6")),
				Default: true,
			},
		}))
	})

	When("a version is invalid", func() {
		It("returns an error", func() {
			m := manifest.Manifest{
				TerraformResources: []manifest.TerraformResource{
					{
						Name:    "terraform",
						Version: "invalid",
						Default: true,
					},
				},
			}

			v, err := m.TerraformVersions()
			Expect(err).To(MatchError(`failed to parse terraform version: Malformed version: invalid`))
			Expect(v).To(BeEmpty())
		})
	})

})
