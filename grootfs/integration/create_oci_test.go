package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"syscall"

	digestpkg "github.com/opencontainers/go-digest"

	"code.cloudfoundry.org/grootfs/groot"
	runnerpkg "code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Create with OCI images", func() {
	var (
		baseImageURL string
		workDir      string
		runner       runnerpkg.Runner
	)

	BeforeEach(func() {
		var err error
		workDir, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		baseImageURL = fmt.Sprintf("oci:///%s/assets/oci-test-image/grootfs-busybox:latest", workDir)

		initSpec := runnerpkg.InitSpec{UIDMappings: []groot.IDMappingSpec{
			{HostID: GrootUID, NamespaceID: 0, Size: 1},
			{HostID: 100000, NamespaceID: 1, Size: 65000},
		},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: GrootGID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
		}

		Expect(Runner.RunningAsUser(0, 0).InitStore(initSpec)).To(Succeed())
		runner = Runner.SkipInitStore()
	})

	It("creates a root filesystem based on the image provided", func() {
		image, err := runner.Create(groot.CreateSpec{
			BaseImage: baseImageURL,
			ID:        "random-id",
			Mount:     mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(image)).To(Succeed())

		Expect(path.Join(image.Rootfs, "file-1")).To(BeARegularFile())
		Expect(path.Join(image.Rootfs, "file-2")).To(BeARegularFile())
		Expect(path.Join(image.Rootfs, "file-3")).To(BeARegularFile())
	})

	It("saves the image.json to the image folder", func() {
		image, err := runner.Create(groot.CreateSpec{
			BaseImage: baseImageURL,
			ID:        "random-id",
			Mount:     mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(image)).To(Succeed())

		imageJsonPath := path.Join(image.Path, "image.json")
		Expect(imageJsonPath).To(BeARegularFile())

		imageJsonReader, err := os.Open(imageJsonPath)
		Expect(err).ToNot(HaveOccurred())
		var imageJson specsv1.Image
		Expect(json.NewDecoder(imageJsonReader).Decode(&imageJson)).To(Succeed())

		Expect(imageJson.Created.String()).To(Equal("2017-08-02 10:38:44.277669063 +0000 UTC"))
		Expect(imageJson.RootFS.DiffIDs).To(Equal([]digestpkg.Digest{
			digestpkg.NewDigestFromHex("sha256", "1a1021d32ed45a8fbd363739882c98f435dd34a050f8064943a79b9808c0da23"),
			digestpkg.NewDigestFromHex("sha256", "11cbb5fdb554a60aef2f2f9bb8443a171a8dadb7ed1d85e4902c7dc08ce7f15e"),
			digestpkg.NewDigestFromHex("sha256", "861df241050979359154ee2eed2f1213704eae7b05695e8f2897067e5d152d7e"),
			digestpkg.NewDigestFromHex("sha256", "9efe56e4c4179f822c558ebc571f1cb27e93656c6b62c7979ffc066d2e3a17e2"),
		}))
	})

	It("gives any user permission to be inside the container", func() {
		image, err := runner.Create(groot.CreateSpec{
			BaseImage: baseImageURL,
			ID:        "random-id",
			Mount:     mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(image)).To(Succeed())

		cmd := exec.Command(NamespacerBin, image.Rootfs, strconv.Itoa(GrootUID+100), "/bin/ls", "/")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		}
		sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	It("outputs a json with the correct `rootfs` key", func() {
		image, err := runner.Create(groot.CreateSpec{
			BaseImage: baseImageURL,
			ID:        "random-id",
			Mount:     mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(image)).To(Succeed())

		Expect(image.Rootfs).To(Equal(filepath.Join(StorePath, store.ImageDirName, "random-id", "rootfs")))
	})

	It("outputs a json with the correct `config` key", func() {
		image, err := runner.Create(groot.CreateSpec{
			BaseImage: baseImageURL,
			ID:        "random-id",
			Mount:     mountByDefault(),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(Runner.EnsureMounted(image)).To(Succeed())
		Expect(image.Image.RootFS.DiffIDs[0]).To(Equal(digestpkg.NewDigestFromHex("sha256", "1a1021d32ed45a8fbd363739882c98f435dd34a050f8064943a79b9808c0da23")))
	})

	Context("when the image has volumes", func() {
		BeforeEach(func() {
			baseImageURL = fmt.Sprintf("oci:///%s/assets/oci-test-image/with-volume:latest", workDir)
		})

		It("lists the volumes as mounts in the returned spec", func() {
			image, err := runner.Create(groot.CreateSpec{
				BaseImage: baseImageURL,
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			volumeHash := sha256.Sum256([]byte("/foo"))
			mountSourceName := "vol-" + hex.EncodeToString(volumeHash[:32])
			Expect(image.Mounts).To(ContainElement(groot.MountInfo{
				Destination: "/foo",
				Source:      filepath.Join(StorePath, store.ImageDirName, "random-id", mountSourceName),
				Type:        "bind",
				Options:     []string{"bind"},
			}))
		})

		It("create the bind mount source for the volumes", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImage: baseImageURL,
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			volumeHash := sha256.Sum256([]byte("/foo"))
			mountSourceName := "vol-" + hex.EncodeToString(volumeHash[:32])

			Expect(filepath.Join(StorePath, store.ImageDirName, "random-id", mountSourceName)).To(BeADirectory())
		})
	})

	Context("when the image has opaque white outs", func() {
		BeforeEach(func() {
			baseImageURL = fmt.Sprintf("oci:///%s/assets/oci-test-image/opq-whiteouts-busybox:latest", workDir)
		})

		It("empties the folder contents but keeps the dir", func() {
			image, err := runner.Create(groot.CreateSpec{
				BaseImage: baseImageURL,
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())

			whiteoutedDir := path.Join(image.Rootfs, "var")
			Expect(whiteoutedDir).To(BeADirectory())
			contents, err := ioutil.ReadDir(whiteoutedDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(BeEmpty())
		})
	})

	Context("when the image has files with the setuid on", func() {
		It("correctly applies the user bit", func() {
			image, err := runner.Create(groot.CreateSpec{
				BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/garden-busybox:latest", workDir),
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())

			setuidFilePath := path.Join(image.Rootfs, "bin", "busybox")
			stat, err := os.Stat(setuidFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stat.Mode() & os.ModeSetuid).To(Equal(os.ModeSetuid))
		})
	})

	Describe("clean up on create", func() {
		var imageID string

		JustBeforeEach(func() {
			_, err := runner.Create(groot.CreateSpec{
				ID:        "my-busybox",
				BaseImage: baseImageURL,
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(Runner.Delete("my-busybox")).To(Succeed())
			imageID = "random-id"
		})

		AfterEach(func() {
			Expect(Runner.Delete(imageID)).To(Succeed())
		})

		It("cleans up unused layers before create but not the one about to be created", func() {
			createSpec := groot.CreateSpec{
				ID:        "my-empty",
				BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir),
				Mount:     mountByDefault(),
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

			runner := runner.WithClean()
			_, err = runner.Create(groot.CreateSpec{
				ID:        imageID,
				BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir),
				Mount:     mountByDefault(),
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
					ID:        imageID,
					BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir),
					Mount:     mountByDefault(),
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
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     mountByDefault(),
					DiskLimit: 10,
				})
				Expect(err).To(MatchError(ContainSubstring("layers exceed disk quota")))
			})
		})
	})

	Describe("Unpacked layer caching", func() {
		It("caches the unpacked image as a volume", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir),
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).ToNot(HaveOccurred())

			layerSnapshotPath := filepath.Join(StorePath, "volumes", "9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233")
			Expect(ioutil.WriteFile(layerSnapshotPath+"/injected-file", []byte{}, 0666)).To(Succeed())

			image, err := runner.Create(groot.CreateSpec{
				BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/empty:v0.1.1", workDir),
				ID:        "random-id-2",
				Mount:     mountByDefault(),
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())
			Expect(path.Join(image.Rootfs, "injected-file")).To(BeARegularFile())
		})

		Describe("when unpacking the image fails", func() {
			It("deletes the layer volume cache", func() {
				_, err := runner.Create(groot.CreateSpec{
					BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/corrupted:latest", workDir),
					ID:        "random-id-2",
					Mount:     true,
				})

				Expect(err).To(MatchError(ContainSubstring("layer is corrupted")))
				layerSnapshotPath := filepath.Join(StorePath, "volumes", "06c1a80a513da76aee4a197d7807ddbd94e80fc9d669f6cd2c5a97b231cd55ac")
				Expect(layerSnapshotPath).ToNot(BeAnExistingFile())
			})
		})
	})

	Context("when the image does not exist", func() {
		It("returns a useful error", func() {
			_, err := runner.Create(groot.CreateSpec{
				BaseImage: "oci:///cfgarden/sorry-not-here",
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).To(MatchError(ContainSubstring("Image source doesn't exist")))
		})
	})

	Context("when using mappings", func() {
		BeforeEach(func() {
			baseImageURL = fmt.Sprintf("oci:///%s/assets/oci-test-image/grootfs-busybox:latest", workDir)
		})

		It("maps the owners of the files", func() {
			image, err := runner.SkipInitStore().Create(groot.CreateSpec{
				BaseImage: baseImageURL,
				ID:        "random-id",
				Mount:     mountByDefault(),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())

			rootOwnedFile, err := os.Stat(path.Join(image.Rootfs, "/etc"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootOwnedFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootOwnedFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

			user33OwnedFile, err := os.Stat(path.Join(image.Rootfs, "/var/www"))
			Expect(err).NotTo(HaveOccurred())
			Expect(user33OwnedFile.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(99999 + 33)))
			Expect(user33OwnedFile.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(99999 + 33)))
		})
	})

	Context("when the image has files that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = fmt.Sprintf("oci:///%s/assets/oci-test-image/non-writable-file:latest", workDir)
		})

		Context("when providing id mappings", func() {
			It("creates those files", func() {
				image, err := Runner.SkipInitStore().Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.EnsureMounted(image)).To(Succeed())
				Expect(path.Join(image.Rootfs, "test", "hello")).To(BeARegularFile())
			})
		})
	})

	Context("when the image has folders that are not writable to their owner", func() {
		BeforeEach(func() {
			baseImageURL = fmt.Sprintf("oci:///%s/assets/oci-test-image/non-writable-folder:latest", workDir)
		})

		Context("when providing id mappings", func() {
			It("works", func() {
				image, err := runner.Create(groot.CreateSpec{
					BaseImage: baseImageURL,
					ID:        "random-id",
					Mount:     mountByDefault(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(Runner.EnsureMounted(image)).To(Succeed())
				Expect(path.Join(image.Rootfs, "test", "hello")).To(BeARegularFile())
			})
		})
	})

	Context("when --skip-layer-validation flag is passed", func() {
		It("does not validate the checksums for oci image layers", func() {
			image, err := runner.SkipLayerCheckSumValidation().Create(groot.CreateSpec{
				BaseImage: fmt.Sprintf("oci:///%s/assets/oci-test-image/also-corrupted:latest", workDir),
				ID:        "random-id",
				Mount:     mountByDefault(),
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(Runner.EnsureMounted(image)).To(Succeed())
			Expect(filepath.Join(image.Rootfs, "corrupted")).To(BeARegularFile())
		})
	})
})
