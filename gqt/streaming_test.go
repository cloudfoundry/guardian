package gqt_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "code.cloudfoundry.org/guardian/matchers"

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

		client = runner.Start(config)

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

	Describe("StreamOut", func() {
		BeforeEach(func() {
			process, err := container.Run(garden.ProcessSpec{
				User: "root",
				Path: "sh",
				Args: []string{"-c", "mkdir -p /root/documents/some/reports && echo hello > /root/documents/some/reports/test"},
			}, ginkgoIO)

			Expect(err).NotTo(HaveOccurred())
			statusCode, err := process.Wait()

			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(0))
		})

		It("should stream out the files", func() {
			tarStream, err := container.StreamOut(garden.StreamOutSpec{
				Path: "/root/documents/some/reports",
				User: "root",
			})
			Expect(err).NotTo(HaveOccurred())

			tarReader := tar.NewReader(tarStream)

			header, err := tarReader.Next()
			Expect(err).ToNot(HaveOccurred())
			Expect(header.Name).To(Equal("reports/"))

			header, err = tarReader.Next()
			Expect(err).ToNot(HaveOccurred())
			Expect(header.Name).To(Equal("reports/test"))

			buffer := bytes.NewBufferString("")

			_, err = io.Copy(buffer, tarReader)
			Expect(err).NotTo(HaveOccurred())

			Expect(buffer.String()).To(Equal("hello\n"))
		})
	})
})
