package tfproviderfqn_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTfproviderFQN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TfproviderFQN Suite")
}
