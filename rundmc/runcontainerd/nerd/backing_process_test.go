package nerd_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"
	containerd "github.com/containerd/containerd/v2/client"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd/nerdfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BackingProcess", func() {
	var (
		containerdProcess *nerdfakes.FakeProcess
		containerdIO      *nerdfakes.FakeIO
		backingProcess    nerd.BackingProcess
	)

	BeforeEach(func() {
		containerdProcess = new(nerdfakes.FakeProcess)
		containerdIO = new(nerdfakes.FakeIO)
		containerdProcess.IOReturns(containerdIO)
		backingProcess = nerd.NewBackingProcess(lagertest.NewTestLogger("backing-process"), containerdProcess, context.Background())
	})

	Describe("Wait", func() {
		var (
			exitCode   int
			exitCh     chan containerd.ExitStatus
			exitStatus containerd.ExitStatus
			err        error
		)

		BeforeEach(func() {
			exitCh = make(chan containerd.ExitStatus, 1)
			exitStatus = *containerd.NewExitStatus(123, time.Now(), nil)
			containerdProcess.WaitReturns(exitCh, nil)
		})

		JustBeforeEach(func() {
			exitCh <- exitStatus
			exitCode, err = backingProcess.Wait()
		})

		It("returns the exit code", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(123))
		})

		It("waits for IO", func() {
			Expect(containerdIO.WaitCallCount()).To(Equal(1))
		})

		When("waiting for the process fails", func() {
			BeforeEach(func() {
				exitStatus = *containerd.NewExitStatus(9, time.Now(), errors.New("wait-failed"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("wait-failed"))
			})

			It("does not wait for the IO (because it does not make sense)", func() {
				Expect(containerdIO.WaitCallCount()).To(BeZero())
			})
		})
	})
})
