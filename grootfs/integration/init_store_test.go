package integration_test

import (
	"fmt"
	"os"
	"syscall"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InitStore", func() {
	var (
		rootUID   int
		storePath string
		runner    runner.Runner
	)

	BeforeEach(func() {
		integration.SkipIfNonRoot(GrootfsTestUid)
		rootUID = 0
		storePath = fmt.Sprintf("/opt/init_store_%s/grootfs", Runner.Driver)
		runner = Runner.WithStore(storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath))
	})

	Context("when the given store path is mounted", func() {
		Context("and the given driver matches the mounted path", func() {
			It("returns a newly created store path", func() {
				_, err := runner.InitStore()
				Expect(err).NotTo(HaveOccurred())

				Expect(runner.StorePath).To(BeADirectory())

				stat, err := os.Stat(runner.StorePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(rootUID)))
				Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
			})
		})

		Context("and the given driver does not match the mounted path", func() {
			BeforeEach(func() {
				runner = Runner.WithStore("/mnt/ext4/grootfs")
			})

			It("returns an error", func() {
				_, err := runner.InitStore()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Sprintf("Failed to initialize store path %s: filesystem driver requires store filesystem to be %s", runner.StorePath, runner.Driver)))
				Expect(runner.StorePath).ToNot(BeADirectory())
			})
		})
	})

	Context("when the given store path is already initialized", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(runner.StorePath, 0755)).To(Succeed())
		})

		It("returns an error", func() {
			_, err := runner.InitStore()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Store already initialized at path %s", runner.StorePath)))
		})
	})

	Context("when the user is not root", func() {
		It("returns an error", func() {
			_, err := runner.RunningAsUser(GrootUID, GrootUID).InitStore()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("Store %s can only be initialized by Root user", runner.StorePath)))
		})
	})
})
