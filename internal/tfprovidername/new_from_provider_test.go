package tfprovidername_test

import (
	"github.com/cloudfoundry/cloud-service-broker/internal/tfprovidername"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewFromProvider", func() {
	It("can be created from just the type", func() {
		n, err := tfprovidername.NewFromProvider("postgresql")
		Expect(err).NotTo(HaveOccurred())
		Expect(n.String()).To(Equal("registry.terraform.io/hashicorp/postgresql"))
	})

	It("can be created from the namespace and type", func() {
		n, err := tfprovidername.NewFromProvider("cyrilgdn/postgresql")
		Expect(err).NotTo(HaveOccurred())
		Expect(n.String()).To(Equal("registry.terraform.io/cyrilgdn/postgresql"))
	})

	It("can be created from the registry, namespace and type", func() {
		n, err := tfprovidername.NewFromProvider("myreg.mydomain.com/cyrilgdn/postgresql")
		Expect(err).NotTo(HaveOccurred())
		Expect(n.String()).To(Equal("myreg.mydomain.com/cyrilgdn/postgresql"))
	})

	When("the format is invalid", func() {
		It("returns an error", func() {
			n, err := tfprovidername.NewFromProvider("myreg/mydomain.com/cyrilgdn/postgresql")
			Expect(err).To(MatchError("invalid format; valid format is [<HOSTNAME>/]<NAMESPACE>/<TYPE>"))
			Expect(n).To(BeZero())
		})
	})
})
