package integration_test

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create with OCI images", func() {
	var (
		randomImageID string
		baseImageURL  *url.URL
		workDir       string
		runner        runnerpkg.Runner
	)

	BeforeEach(func() {
		var err error
		workDir, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/grootfs-busybox:latest", workDir))

		initSpec := runnerpkg.InitSpec{UIDMappings: []groot.IDMappingSpec{
			{HostID: GrootUID, NamespaceID: 0, Size: 1},
			{HostID: 100000, NamespaceID: 1, Size: 65000},
		},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: GrootGID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
		}

		randomImageID = testhelpers.NewRandomID()
		Expect(Runner.RunningAsUser(0, 0).InitStore(initSpec)).To(Succeed())
		runner = Runner.SkipInitStore()
	})

	It("creates a root filesystem based on the image provided", func() {
		containerSpec, err := runner.Create(groot.CreateSpec{
			BaseImageURL: baseImageURL,
			ID:           randomImageID,
			Mount:        mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

		Expect(path.Join(containerSpec.Root.Path, "file-1")).To(BeARegularFile())
		Expect(path.Join(containerSpec.Root.Path, "file-2")).To(BeARegularFile())
		Expect(path.Join(containerSpec.Root.Path, "file-3")).To(BeARegularFile())
	})

	It("gives any user permission to be inside the container", func() {
		containerSpec, err := runner.Create(groot.CreateSpec{
			BaseImageURL: baseImageURL,
			ID:           randomImageID,
			Mount:        mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

		cmd := exec.Command(NamespacerBin, containerSpec.Root.Path, strconv.Itoa(GrootUID+100), "/bin/ls", "/")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		}
		sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	It("outputs a json with the correct `rootfs` key", func() {
		containerSpec, err := runner.Create(groot.CreateSpec{
			BaseImageURL: baseImageURL,
			ID:           randomImageID,
			Mount:        mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

		Expect(containerSpec.Root.Path).To(Equal(filepath.Join(StorePath, store.ImageDirName, randomImageID, "rootfs")))
	})

	Context("when the image has cloudfoundry annotations", func() {
		Describe("org.cloudfoundry.experimental.image.base-directory", func() {
			BeforeEach(func() {
				integration.SkipIfNonRoot(GrootfsTestUid)
				baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/cloudfoundry.experimental.image.base-directory:latest", workDir))
			})

			It("untars the layer in the specified folder", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        true,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(path.Join(containerSpec.Root.Path, "home", "example")).To(BeARegularFile())
			})
		})
	})

	Context("when the image has volumes", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/with-volume:latest", workDir))
		})

		It("lists the volumes as mounts in the returned spec", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			volumeHash := sha256.Sum256([]byte("/foo"))
			mountSourceName := "vol-" + hex.EncodeToString(volumeHash[:32])
			Expect(containerSpec.Mounts).To(ContainElement(specs.Mount{
				Destination: "/foo",
				Source:      filepath.Join(StorePath, store.ImageDirName, randomImageID, mountSourceName),
				Type:        "bind",
				Options:     []string{"bind"},
			}))
		})

		It("create the bind mount source for the volumes", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			volumeHash := sha256.Sum256([]byte("/foo"))
			mountSourceName := "vol-" + hex.EncodeToString(volumeHash[:32])

			Expect(filepath.Join(StorePath, store.ImageDirName, randomImageID, mountSourceName)).To(BeADirectory())
		})
	})

	Context("when the image has opaque white outs", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/opq-whiteouts-busybox:latest", workDir))
		})

		It("empties the folder contents but keeps the dir", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

			whiteoutedDir := path.Join(containerSpec.Root.Path, "var")
			Expect(whiteoutedDir).To(BeADirectory())
			contents, err := ioutil.ReadDir(whiteoutedDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(BeEmpty())
		})
	})

	Context("when the image has files with the setuid on", func() {
		It("correctly applies the user bit", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/garden-busybox:latest", workDir)),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

			setuidFilePath := path.Join(containerSpec.Root.Path, "bin", "busybox")
			stat, err := os.Stat(setuidFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode() & os.ModeSetuid).To(Equal(os.ModeSetuid))
		})
	})

	Describe("clean up on create", func() {
		JustBeforeEach(func() {
			_, err := runner.Create(groot.CreateSpec{
				ID:           "my-busybox",
				BaseImageURL: baseImageURL,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
		})

		AfterEach(func() {
			Expect(Runner.Delete(randomImageID)).To(Succeed())
		})

		It("cleans up unused layers before create but not the one about to be created", func() {
			createSpec := groot.CreateSpec{
				ID:           "my-empty",
				BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir)),
				Mount:        mountByDefault(),
			}
			_, err := runner.Create(createSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.Delete("my-empty")).To(Succeed())

			layerPath := filepath.Join(StorePath, store.VolumesDirName, "9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233")
			stat, err := os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			preLayerTimestamp := stat.ModTime()

			preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(preContents).To(HaveLen(6))

			runner = runner.WithClean()
			_, err = runner.Create(groot.CreateSpec{
				ID:           randomImageID,
				BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir)),
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(afterContents).To(HaveLen(2))

			Expect(filepath.Join(StorePath, store.VolumesDirName, "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5")).To(BeADirectory())
			Expect(filepath.Join(StorePath, store.VolumesDirName, "9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233")).To(BeADirectory())

			stat, err = os.Stat(layerPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.ModTime()).To(Equal(preLayerTimestamp))
		})

		Context("when no-clean flag is set", func() {
			It("does not clean up unused layers", func() {
				preContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(preContents).To(HaveLen(4))

				_, err = runner.WithNoClean().Create(groot.CreateSpec{
					ID:           randomImageID,
					BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir)),
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())

				afterContents, err := ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
				Expect(err).NotTo(HaveOccurred())
				Expect(afterContents).To(HaveLen(6))
			})
		})
	})

	Context("when image size exceeds disk quota", func() {
		Context("when the image is accounted for in the quota", func() {
			It("returns an error", func() {
				_, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
					DiskLimit:    10,
				})
				Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
			})
		})
	})

	Describe("Unpacked layer caching", func() {
		It("caches the unpacked image as a volume", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir)),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).ToNot(HaveOccurred())

			layerSnapshotPath := filepath.Join(StorePath, "volumes", "9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233")
			Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir)),
				ID:           testhelpers.NewRandomID(),
				Mount:        mountByDefault(),
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())
			Expect(path.Join(containerSpec.Root.Path, "injected-file")).To(BeARegularFile())
		})

		Describe("when unpacking the image fails", func() {
			It("deletes the layer volume cache", func() {
				_, err := runner.Create(groot.CreateSpec{
					BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/corrupted:latest", workDir)),
					ID:           testhelpers.NewRandomID(),
					Mount:        true,
				})

				Expect(err).To(MatchError(ContainSubstring("layerID digest mismatch")))
				layerSnapshotPath := filepath.Join(StorePath, "volumes", "06c1a80a513da76aee4a197d7807ddbd94e80fc9d669f6cd2c5a97b231cd55ac")
				Expect(layerSnapshotPath).ToNot(BeAnExistingFile())
			})
		})
	})

	Context("when the image does not exist", func() {
		It("returns a useful error", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL("oci:///cfgarden/sorry-not-here"),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).To(MatchError(ContainSubstring("Image source doesn't exist")))
		})
	})

	Context("when using mappings", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/grootfs-busybox:latest", workDir))
		})

		It("maps the owners of the files", func() {
			containerSpec, err := runner.SkipInitStore().Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

			rootOwnedFile, err := os.Stat(path.Join(containerSpec.Root.Path, "/etc"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootOwnedFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootOwnedFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

			user33OwnedFile, err := os.Stat(path.Join(containerSpec.Root.Path, "/var/www"))
			Expect(err).NotTo(HaveOccurred())
			Expect(user33OwnedFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(99999 + 33)))
			Expect(user33OwnedFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(99999 + 33)))
		})
	})

	Context("when a layer is an uncompressed blob", func() {
		BeforeEach(func() {
			integration.SkipIfNonRoot(GrootfsTestUid)
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/tar-layer:latest", workDir))
		})

		It("is readable after image creation", func() {
			containerSpec, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			filePath := path.Join(containerSpec.Root.Path, "pokemon.txt")
			Expect(strings.TrimSpace(readFile(filePath))).To(Equal("pikachu"))
		})
	})

	Context("when the image has files that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/non-writable-file:latest", workDir))
		})

		Context("when providing id mappings", func() {
			It("creates those files", func() {
				containerSpec, err := Runner.SkipInitStore().Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())
				Expect(path.Join(containerSpec.Root.Path, "test", "hello")).To(BeARegularFile())
			})
		})
	})

	Context("when the image has folders that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/non-writable-folder:latest", workDir))
		})

		Context("when providing id mappings", func() {
			It("works", func() {
				containerSpec, err := runner.Create(groot.CreateSpec{
					BaseImageURL: baseImageURL,
					ID:           randomImageID,
					Mount:        mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())
				Expect(path.Join(containerSpec.Root.Path, "test", "hello")).To(BeARegularFile())
			})
		})
	})

	Context("when --skip-layer-validation flag is passed", func() {
		It("does not validate the checksums for oci image layers", func() {
			containerSpec, err := runner.SkipLayerCheckSumValidation().Create(groot.CreateSpec{
				BaseImageURL: integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/also-corrupted:latest", workDir)),
				ID:           randomImageID,
				Mount:        mountByDefault(),
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(containerSpec)).To(Succeed())

			Expect(filepath.Join(containerSpec.Root.Path, "corrupted")).To(BeARegularFile())
		})
	})

	Context("with a remote layer in an image", func() {
		var blobstore *http.Server
		var blobstoreStopSignal chan struct{}

		BeforeEach(func() {
			baseImageURL = integration.String2URL(fmt.Sprintf("oci:///%s/assets/oci-test-image/garden-busybox-remote:latest", workDir))
			blobstore, blobstoreStopSignal = startFakeBlobstore(workDir)
		})

		AfterEach(func() {
			blobstore.Close()
			<-blobstoreStopSignal
		})

		It("creates an image", func() {
			cfg := config.Config{
				Create: config.Create{
					RemoteLayerClientCertificatesPath: "assets/certs",
				},
			}
			Expect(runner.SetConfig(cfg)).To(Succeed())

			image, err := runner.Create(groot.CreateSpec{
				BaseImageURL: baseImageURL,
				ID:           "random-id",
				Mount:        mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.EnsureMounted(image)).To(Succeed())
		})

	})
})

func startFakeBlobstore(workDir string) (*http.Server, chan struct{}) {
	certBytes, err := ioutil.ReadFile("assets/certs/cert.cert")
	Expect(err).NotTo(HaveOccurred())

	clientCertPool := x509.NewCertPool()
	Expect(clientCertPool.AppendCertsFromPEM(certBytes)).To(BeTrue())

	tlsConfig := &tls.Config{
		// Reject any TLS certificate that cannot be validated
		ClientAuth: tls.RequireAndVerifyClientCert,
		// Ensure that we only use our "CA" to validate certificates
		ClientCAs: clientCertPool,
	}

	tlsConfig.BuildNameToCertificate()
	fs := http.FileServer(http.Dir(fmt.Sprintf("/%s/assets/remote-layers/garden-busybox-remote", workDir)))
	http.Handle("/", fs)

	httpServer := &http.Server{
		Addr:      ":12000",
		TLSConfig: tlsConfig,
	}

	blobstoreStopSignal := make(chan struct{}, 1)
	go func() {
		httpServer.ListenAndServeTLS("assets/certs/cert.cert", "assets/certs/cert.key")
		blobstoreStopSignal <- struct{}{}
	}()

	return httpServer, blobstoreStopSignal
}

func readFile(name string) string {
	content, err := ioutil.ReadFile(name)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}
