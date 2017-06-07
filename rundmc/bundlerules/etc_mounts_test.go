package bundlerules_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	fakes "code.cloudfoundry.org/guardian/rundmc/bundlerules/bundlerulesfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("EtcMounts", func() {
	var (
		chowner      *fakes.FakeChowner
		containerDir string
		rule         bundlerules.EtcMounts

		bndl goci.Bndl

		transformedBndl goci.Bndl
		applyErr        error
	)

	BeforeEach(func() {
		var err error
		containerDir, err = ioutil.TempDir("", "bundlerules-tests")
		Expect(err).NotTo(HaveOccurred())
		chowner = new(fakes.FakeChowner)
		rule = bundlerules.EtcMounts{Chowner: chowner}

		bndl = goci.Bndl{Spec: specs.Spec{
			Version: "some-version",
			Linux: &specs.Linux{
				UIDMappings: []specs.LinuxIDMapping{
					{
						ContainerID: 1,
						HostID:      875,
						Size:        1,
					},
					{
						ContainerID: 0,
						HostID:      875,
						Size:        1,
					},
					{
						ContainerID: 2,
						HostID:      875,
						Size:        1,
					},
				},
				GIDMappings: []specs.LinuxIDMapping{
					{
						ContainerID: 1,
						HostID:      875,
						Size:        1,
					},
					{
						ContainerID: 0,
						HostID:      876,
						Size:        1,
					},
					{
						ContainerID: 2,
						HostID:      875,
						Size:        1,
					},
				},
			},
		}}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(containerDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		transformedBndl, applyErr = rule.Apply(bndl, gardener.DesiredContainerSpec{}, containerDir)
	})

	It("returns no error", func() {
		Expect(applyErr).NotTo(HaveOccurred())
	})

	It("creates an empty hosts file in the container dir", func() {
		Expect(filepath.Join(containerDir, "hosts")).To(BeARegularFile())
	})

	It("creates an empty resolv.conf file in the container dir", func() {
		Expect(filepath.Join(containerDir, "resolv.conf")).To(BeARegularFile())
	})

	It("chowns the hosts and resolv.conf files to container root", func() {
		Expect(chowner.ChownCallCount()).To(Equal(2))
		path, uid, gid := chowner.ChownArgsForCall(0)
		Expect(path).To(Equal(filepath.Join(containerDir, "hosts")))
		Expect(uid).To(Equal(875))
		Expect(gid).To(Equal(876))

		path, uid, gid = chowner.ChownArgsForCall(1)
		Expect(path).To(Equal(filepath.Join(containerDir, "resolv.conf")))
		Expect(uid).To(Equal(875))
		Expect(gid).To(Equal(876))
	})

	It("returns the bundle with the two mounts added", func() {
		Expect(transformedBndl).To(Equal(bndl.WithMounts(
			specs.Mount{
				Destination: "/etc/hosts",
				Source:      filepath.Join(containerDir, "hosts"),
				Type:        "bind",
				Options:     []string{"bind"},
			},
			specs.Mount{
				Destination: "/etc/resolv.conf",
				Source:      filepath.Join(containerDir, "resolv.conf"),
				Type:        "bind",
				Options:     []string{"bind"},
			},
		)))
	})

	Context("when chowning the hosts file fails", func() {
		BeforeEach(func() {
			chowner.ChownStub = func(path string, _, _ int) error {
				if path == filepath.Join(containerDir, "hosts") {
					return errors.New("fail")
				}
				return nil
			}
		})

		It("returns an error", func() {
			Expect(applyErr).To(HaveOccurred())
		})
	})

	Context("when chowning the resolv.conf file fails", func() {
		BeforeEach(func() {
			chowner.ChownStub = func(path string, _, _ int) error {
				if path == filepath.Join(containerDir, "resolv.conf") {
					return errors.New("fail")
				}
				return nil
			}
		})

		It("returns an error", func() {
			Expect(applyErr).To(HaveOccurred())
		})
	})

	Context("when there is no UID mapping for container root", func() {
		BeforeEach(func() {
			bndl.Spec.Linux.UIDMappings = nil
		})

		It("chowns the hosts and resolv.conf files to UID 0 and the mapped GID", func() {
			Expect(chowner.ChownCallCount()).To(Equal(2))
			path, uid, gid := chowner.ChownArgsForCall(0)
			Expect(path).To(Equal(filepath.Join(containerDir, "hosts")))
			Expect(uid).To(Equal(0))
			Expect(gid).To(Equal(876))

			path, uid, gid = chowner.ChownArgsForCall(1)
			Expect(path).To(Equal(filepath.Join(containerDir, "resolv.conf")))
			Expect(uid).To(Equal(0))
			Expect(gid).To(Equal(876))
		})
	})

	Context("when there is no GID mapping for container root", func() {
		BeforeEach(func() {
			bndl.Spec.Linux.GIDMappings = nil
		})

		It("chowns the hosts and resolv.conf files to the mapped user and GID 0", func() {
			Expect(chowner.ChownCallCount()).To(Equal(2))
			path, uid, gid := chowner.ChownArgsForCall(0)
			Expect(path).To(Equal(filepath.Join(containerDir, "hosts")))
			Expect(uid).To(Equal(875))
			Expect(gid).To(Equal(0))

			path, uid, gid = chowner.ChownArgsForCall(1)
			Expect(path).To(Equal(filepath.Join(containerDir, "resolv.conf")))
			Expect(uid).To(Equal(875))
			Expect(gid).To(Equal(0))
		})
	})
})
