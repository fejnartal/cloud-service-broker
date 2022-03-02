package tfproviderfqn_test

import (
	"github.com/cloudfoundry/cloud-service-broker/internal/tfproviderfqn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("New", func() {
	When("only a name is provided", func() {
		It("is created from the name", func() {
			n, err := tfproviderfqn.New("terraform-provider-mysql", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(n.String()).To(Equal("registry.terraform.io/hashicorp/mysql"))
		})
	})

	When("a provider field is specified", func() {
		It("it is created from the provider field", func() {
			n, err := tfproviderfqn.New("postgresql", "cyrilgdn/postgresql")
			Expect(err).NotTo(HaveOccurred())
			Expect(n.String()).To(Equal("registry.terraform.io/cyrilgdn/postgresql"))
		})
	})
})
