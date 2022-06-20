package packer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("readCachePath()", func() {
	BeforeEach(func() {
		viper.Reset()
	})

	When("$HOME exists", func() {
		It("is $HOME/.csb-pak-cache", func() {
			viper.Set(userHomeDir, "random-home-dir")
			Expect(readCachePath()).To(Equal("random-home-dir/.csb-pak-cache"))
		})
	})

	When("$PAK_BUILD_CACHE_PATH is set to a value", func() {
		It("is $PAK_BUILD_CACHE_PATH", func() {
			viper.Set(userHomeDir, "random-home-dir")
			viper.Set(pakCachePath, "random-cache-dir")
			Expect(readCachePath()).To(Equal("random-cache-dir"))
		})
	})

	When("$PAK_BUILD_CACHE_PATH is set and empty", func() {
		It("is empty", func() {
			viper.Set(userHomeDir, "random-home-dir")
			viper.Set(pakCachePath, "")
			Expect(readCachePath()).To(BeEmpty())
		})
	})

	When("neither $HOME nor $PAK_BUILD_CACHE_PATH have values", func() {
		It("is empty", func() {
			viper.Set(userHomeDir, "")
			viper.Set(pakCachePath, "")
			Expect(readCachePath()).To(BeEmpty())
		})
	})
})
