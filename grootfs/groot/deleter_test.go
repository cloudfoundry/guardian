package groot_test

import (
	"errors"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/st3v/glager"
)

var _ = Describe("Deleter", func() {
	var (
		fakeImageCloner       *grootfakes.FakeImageCloner
		fakeDependencyManager *grootfakes.FakeDependencyManager
		fakeMetricsEmitter    *grootfakes.FakeMetricsEmitter
		deleter               *groot.Deleter
		logger                lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		fakeDependencyManager = new(grootfakes.FakeDependencyManager)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)

		deleter = groot.IamDeleter(fakeImageCloner, fakeDependencyManager, fakeMetricsEmitter)
		logger = lagertest.NewTestLogger("deleter")
	})

	Describe("Delete", func() {
		It("destroys a image", func() {
			Expect(deleter.Delete(logger, "some-id")).To(Succeed())

			_, imageId := fakeImageCloner.DestroyArgsForCall(0)
			Expect(imageId).To(Equal("some-id"))
		})

		It("deregisters image dependencies", func() {
			Expect(deleter.Delete(logger, "some-id")).To(Succeed())
			Expect(fakeDependencyManager.DeregisterCallCount()).To(Equal(1))
		})

		Context("when destroying a image fails", func() {
			BeforeEach(func() {
				fakeImageCloner.DestroyReturns(errors.New("failed to destroy image"))
			})

			It("returns an error", func() {
				Expect(deleter.Delete(logger, "some-id")).To(MatchError(ContainSubstring("failed to destroy image")))
			})
		})

		It("emits metrics for deletion", func() {
			Expect(deleter.Delete(logger, "some-id")).To(Succeed())

			Expect(fakeMetricsEmitter.EmitDurationCallCount()).To(Equal(1))
			name, duration := fakeMetricsEmitter.EmitDurationArgsForCall(0)
			Expect(name).To(Equal(groot.MetricImageDeletionTime))
			Expect(duration).NotTo(BeZero())
		})

		Context("when emitting metrics fails", func() {
			var emitError error

			BeforeEach(func() {
				emitError = errors.New("failed to emit metric")
				fakeMetricsEmitter.EmitDurationReturns(emitError)
			})

			It("does not fail but log the error", func() {
				Expect(deleter.Delete(logger, "some-id")).To(Succeed())

				Expect(logger).To(ContainSequence(
					Error(
						emitError,
						Message("deleter.groot-deleting.failed-to-emit-metric"),
						Data(
							"key", groot.MetricImageDeletionTime,
						),
					),
				))
			})
		})

		Context("when an emitter is not passed", func() {
			BeforeEach(func() {
				deleter = groot.IamDeleter(fakeImageCloner, fakeDependencyManager, nil)
			})

			It("does not expode", func() {
				Expect(deleter.Delete(logger, "some-id")).To(Succeed())
			})
		})
	})
})
