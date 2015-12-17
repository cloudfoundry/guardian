package gqt_test

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/cloudfoundry-incubator/guardian/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	archiver "github.com/pivotal-golang/archiver/extractor/test_helper"
)

var _ = Describe("Streaming", func() {
	var (
		client    *runner.RunningGarden
		container garden.Container
	)

	BeforeEach(func() {
		var err error

		client = startGarden()

		container, err = client.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("StreamIn", func() {
		var tarStream io.Reader

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir("", "some-temp-dir-parent")
			Expect(err).ToNot(HaveOccurred())

			tgzPath := filepath.Join(tmpdir, "some.tgz")

			archiver.CreateTarGZArchive(
				tgzPath,
				[]archiver.ArchiveFile{
					{
						Name: "./some-temp-dir",
						Dir:  true,
					},
					{
						Name: "./some-temp-dir/some-temp-file",
						Body: "some-body",
					},
				},
			)

			tgz, err := os.Open(tgzPath)
			Expect(err).ToNot(HaveOccurred())

			tarStream, err = gzip.NewReader(tgz)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should stream in the files", func() {
			Expect(container.StreamIn(garden.StreamInSpec{
				Path:      "/root/test",
				User:      "root",
				TarStream: tarStream,
			})).To(Succeed())

			Expect(container).To(HaveFile("/root/test/some-temp-dir"))
			Expect(container).To(HaveFile("/root/test/some-temp-dir/some-temp-file"))
		})
	})
})
