package lustre_test

import (
	"hpdd/test/harness"
	"hpdd/test/log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLustre(t *testing.T) {
	BeforeSuite(func() {
		log.AddDebugLogger(&log.ClosingGinkgoWriter{GinkgoWriter})
		if err := harness.Setup(); err != nil {
			panic(err)
		}
	})

	AfterSuite(func() {
		if err := harness.Teardown(); err != nil {
			panic(err)
		}
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "Lustre Suite")
}
