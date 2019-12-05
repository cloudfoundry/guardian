package gqt_test

import (
	"sync"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mass delete", func() {
	var (
		numberOfContainers int
		client             *runner.RunningGarden
	)

	BeforeEach(func() {
		config.NetworkPluginBin = "/bin/true"
		client = runner.Start(config)
		numberOfContainers = 1000
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	FIt("does not fail on concurrent delete", func() {
		handles, err := createContainers(client, numberOfContainers)
		Expect(err).NotTo(HaveOccurred())
		handlesLen := len(handles)

		errorHandlingWg := sync.WaitGroup{}
		errorHandlingWg.Add(1)
		var deleteErrors *multierror.Error
		errorsChan := make(chan error)
		go func() {
			defer errorHandlingWg.Done()

			for e := range errorsChan {
				deleteErrors = multierror.Append(deleteErrors, e)
			}
		}()

		wg := sync.WaitGroup{}
		wg.Add(handlesLen)
		for i := 0; i < handlesLen; i++ {
			go func(index int) {
				defer wg.Done()

				err := client.Destroy(handles[index])
				if err != nil {
					errorsChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errorsChan)

		errorHandlingWg.Wait()

		Expect(deleteErrors.ErrorOrNil()).NotTo(HaveOccurred())
	})

})

func createContainers(client *runner.RunningGarden, numberOfContainers int) ([]string, error) {
	errorHandlingWg := sync.WaitGroup{}
	errorHandlingWg.Add(1)
	var createErrors *multierror.Error
	errorsChan := make(chan error)
	go func() {
		defer errorHandlingWg.Done()

		for e := range errorsChan {
			createErrors = multierror.Append(createErrors, e)
		}
	}()

	handlesHandlingWg := sync.WaitGroup{}
	handlesHandlingWg.Add(1)
	handles := []string{}
	handlesChan := make(chan string)
	go func() {
		defer handlesHandlingWg.Done()

		for h := range handlesChan {
			handles = append(handles, h)
		}
	}()

	batchSize := 100
	batches := numberOfContainers / batchSize

	wg := sync.WaitGroup{}
	wg.Add(batches)
	for i := 0; i < batches; i++ {
		go func() {
			defer wg.Done()

			for i := 0; i < batchSize; i++ {
				c, err := client.Create(garden.ContainerSpec{})
				if err != nil {
					errorsChan <- err
					return
				}
				handlesChan <- c.Handle()
			}
		}()
	}

	wg.Wait()

	close(errorsChan)
	errorHandlingWg.Wait()

	close(handlesChan)
	handlesHandlingWg.Wait()

	return handles, createErrors.ErrorOrNil()
}
