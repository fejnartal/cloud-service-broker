package manifest_test

import (
	"github.com/cloudfoundry/cloud-service-broker/internal/brokerpak/manifest"
	"github.com/cloudfoundry/cloud-service-broker/internal/tfproviderfqn"
	"github.com/hashicorp/go-version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TerraformProviders", func() {
	It("returns a slice of Terraform Providers", func() {
		m, err := manifest.Parse(fakeManifest(withAdditionalEntry("terraform_binaries", map[string]interface{}{
			"name":     "terraform-provider-postgresql",
			"version":  "1.2.3",
			"provider": "cyrilgdn/postgresql",
		})))
		Expect(err).NotTo(HaveOccurred())

		p, err := m.TerraformProviders()
		Expect(err).NotTo(HaveOccurred())

		Expect(p).To(Equal([]manifest.TerraformProvider{
			{
				Name:    "terraform-provider-random",
				Version: version.Must(version.NewVersion("3.1.0")),
				Provider: tfproviderfqn.TfProviderFQN{
					Hostname:  "registry.terraform.io",
					Namespace: "hashicorp",
					Type:      "random",
				},
			},
			{
				Name:    "terraform-provider-postgresql",
				Version: version.Must(version.NewVersion("1.2.3")),
				Provider: tfproviderfqn.TfProviderFQN{
					Hostname:  "registry.terraform.io",
					Namespace: "cyrilgdn",
					Type:      "postgresql",
				},
			},
		}))
	})

	When("a version number is invalid", func() {
		It("returns an error", func() {
			m, err := manifest.Parse(fakeManifest(withAdditionalEntry("terraform_binaries", map[string]interface{}{
				"name":     "terraform-provider-postgresql",
				"version":  "invalid",
				"provider": "cyrilgdn/postgresql",
			})))
			Expect(err).NotTo(HaveOccurred())

			p, err := m.TerraformProviders()
			Expect(err).To(MatchError(`error parsing version for terraform provider "terraform-provider-postgresql": Malformed version: invalid`))
			Expect(p).To(BeEmpty())
		})
	})
})
