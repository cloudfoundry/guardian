package streamer_test

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"path"

	streamerpkg "code.cloudfoundry.org/grootfs/cloner/streamer"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TarStreamer", func() {
	Describe("Stream", func() {
		var (
			sourcePath  string
			tarStreamer *streamerpkg.TarStreamer
			logger      *lagertest.TestLogger
		)

		BeforeEach(func() {
			var err error

			sourcePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(path.Join(sourcePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

			tarStreamer = streamerpkg.NewTarStreamer()
			logger = lagertest.NewTestLogger("TarStreamer")
		})

		It("returns the contents of the source directory as a Tar stream", func() {
			stream, _, err := tarStreamer.Stream(logger, sourcePath)
			Expect(err).ToNot(HaveOccurred())

			entries := streamTar(tar.NewReader(stream))
			Expect(entries).To(HaveLen(2))
			Expect(entries[1].header.Name).To(Equal("./a_file"))
			Expect(entries[1].header.Mode).To(Equal(int64(0600)))
			Expect(string(entries[1].contents)).To(Equal("hello-world"))
		})

		Context("when tar does not exist", func() {
			It("returns an error", func() {
				streamerpkg.TarBin = "non-existent-tar"

				_, _, err := tarStreamer.Stream(logger, sourcePath)
				Expect(err).To(MatchError(ContainSubstring("starting command")))
			})
		})
	})
})

type tarEntry struct {
	header   *tar.Header
	contents []byte
}

func streamTar(r *tar.Reader) []tarEntry {
	l := []tarEntry{}
	for {
		header, err := r.Next()
		if err != nil {
			Expect(err).To(Equal(io.EOF))
			return l
		}

		contents := make([]byte, header.Size)
		r.Read(contents)
		l = append(l, tarEntry{
			header:   header,
			contents: contents,
		})
	}
}
