package rundmc_test

import (
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("exitstore", func() {
	var exitStore rundmc.ExitStore

	BeforeEach(func() {
		exitStore = rundmc.NewExitStore()
	})

	It("does not block in wait if the handle has not been stored", func() {
		ch := make(chan struct{})
		go func() {
			exitStore.Wait("handle")
			close(ch)
		}()

		Eventually(ch).Should(BeClosed())
	})

	It("blocks in wait until the stored handle is written", func() {
		exitCh := make(chan struct{})
		exitStore.Store("handle", exitCh)

		ch := make(chan struct{})
		go func() {
			exitStore.Wait("handle")
			close(ch)
		}()

		Consistently(ch).ShouldNot(BeClosed())
		close(exitCh)
		Eventually(ch).Should(BeClosed())
	})

	It("allows multiple waits", func() {
		exitCh := make(chan struct{})
		exitStore.Store("handle", exitCh)

		wait1 := make(chan struct{})
		go func() {
			exitStore.Wait("handle")
			close(wait1)
		}()

		wait2 := make(chan struct{})
		go func() {
			exitStore.Wait("handle")
			close(wait2)
		}()

		Consistently(wait1).ShouldNot(BeClosed())
		Consistently(wait2).ShouldNot(BeClosed())
		close(exitCh)
		Eventually(wait1).Should(BeClosed())
		Eventually(wait2).Should(BeClosed())
	})

	It("allows store to run even while wait is ongoing", func() {
		exitCh := make(chan struct{})
		exitStore.Store("handle", exitCh)
		go exitStore.Wait("handle")

		defer close(exitCh)

		stored := make(chan struct{})
		go func() {
			exitStore.Store("blahblah", nil)
			close(stored)
		}()

		Eventually(stored).Should(BeClosed())
	})

	It("cleans up channels when Unstore is called", func() {
		exitCh := make(chan struct{})

		exitStore.Store("handle", exitCh)
		exitStore.Unstore("handle")

		ch := make(chan struct{})
		go func() {
			exitStore.Wait("handle")
			close(ch)
		}()

		Eventually(ch).Should(BeClosed())
	})
})
