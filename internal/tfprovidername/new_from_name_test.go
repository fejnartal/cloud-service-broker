package tfprovidername_test

import (
	"github.com/cloudfoundry/cloud-service-broker/internal/tfprovidername"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewFromName", func() {
	It("can be created from a name", func() {
		n, err := tfprovidername.NewFromName("terraform-provider-mysql")
		Expect(err).NotTo(HaveOccurred())
		Expect(n.String()).To(Equal("registry.terraform.io/hashicorp/mysql"))
	})

	When("the name has the wrong prefix", func() {
		It("returns an error", func() {
			n, err := tfprovidername.NewFromName("mysql")
			Expect(err).To(MatchError("name must have prefix: terraform-provider-"))
			Expect(n).To(BeZero())
		})
	})
})
