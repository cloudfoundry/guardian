package nerd_test

import (
	"context"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nerd Stopper", func() {
	var (
		nerdStopper *nerd.NerdStopper
		stopErr     error
	)

	BeforeEach(func() {
		nerdStopper = nerd.NewNerdStopper(containerdClient)
	})

	JustBeforeEach(func() {
		stopErr = nerdStopper.Stop()
	})

	It("succeeds", func() {
		Expect(stopErr).NotTo(HaveOccurred())
	})

	It("closes the underlying GRPC connection", func() {
		_, err := containerdClient.Containers(context.Background())
		Expect(err).To(MatchError(ContainSubstring("grpc")))
	})

})
