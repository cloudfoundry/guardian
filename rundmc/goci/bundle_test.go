package goci_test

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Bundle", func() {
	var initialBundle goci.Bndl
	var returnedBundle goci.Bndl

	BeforeEach(func() {
		initialBundle = goci.Bundle()
	})

	It("specifies the correct version", func() {
		Expect(initialBundle.Spec.Version).To(Equal("1.0.0-rc3"))
	})

	Describe("WithHostname", func() {
		It("sets the Hostname in the bundle", func() {
			returnedBundle := initialBundle.WithHostname("hostname")
			Expect(returnedBundle.Hostname()).To(Equal("hostname"))
		})
	})

	Describe("WithCapabilities", func() {
		It("adds capabilities to the bundle", func() {
			returnedBundle := initialBundle.WithCapabilities("growtulips", "waterspuds")
			Expect(returnedBundle.Capabilities()).To(ContainElement("growtulips"))
			Expect(returnedBundle.Capabilities()).To(ContainElement("waterspuds"))
		})

		It("adds capabilities to the spec's effective, bounding, inheritable, and permitted sets", func() {
			returnedBundle := initialBundle.WithCapabilities("growtulips", "waterspuds")
			Expect(returnedBundle.Spec.Process.Capabilities.Effective).To(BeEmpty())
			Expect(returnedBundle.Spec.Process.Capabilities.Bounding).To(ConsistOf("growtulips", "waterspuds"))
			Expect(returnedBundle.Spec.Process.Capabilities.Inheritable).To(ConsistOf("growtulips", "waterspuds"))
			Expect(returnedBundle.Spec.Process.Capabilities.Permitted).To(ConsistOf("growtulips", "waterspuds"))
			Expect(returnedBundle.Spec.Process.Capabilities.Ambient).To(BeEmpty())
		})

		It("does not modify the initial bundle", func() {
			returnedBundle := initialBundle.WithCapabilities("growtulips", "waterspuds")
			Expect(returnedBundle).NotTo(Equal(initialBundle))
		})
	})

	Describe("Capabilities", func() {
		Context("when the process has no capbilities", func() {
			It("returns an empty list", func() {
				Expect(initialBundle.Capabilities()).To(BeEmpty())
			})
		})
	})

	Describe("WithProcess", func() {
		It("adds the process to the bundle", func() {
			returnedBundle := initialBundle.WithProcess(goci.Process("echo", "foo"))
			Expect(returnedBundle.Process()).To(Equal(specs.Process{Args: []string{"echo", "foo"}}))
		})

		It("sets the CWD to / by default", func() {
			returnedBundle := initialBundle.WithProcess(goci.Process("echo", "foo"))
			Expect(returnedBundle.Process()).To(Equal(specs.Process{Args: []string{"echo", "foo"}}))
		})

		It("does not modify the initial bundle", func() {
			returnedBundle := initialBundle.WithProcess(goci.Process("echo", "foo"))
			Expect(returnedBundle).NotTo(Equal(initialBundle))
		})
	})

	Describe("WithRootFS", func() {
		It("sets the rootfs path in the spec", func() {
			returnedBundle := initialBundle.WithRootFS("/foo/bar/baz")
			Expect(returnedBundle.RootFS()).To(Equal("/foo/bar/baz"))
		})
	})

	Describe("WithPrestartHooks", func() {
		It("adds the hook to the runtime spec", func() {
			returnedBundle := initialBundle.WithPrestartHooks(specs.Hook{
				Path: "foo",
				Args: []string{"bar", "baz"},
			})

			Expect(returnedBundle.PrestartHooks()).To(Equal([]specs.Hook{{
				Path: "foo",
				Args: []string{"bar", "baz"},
			}}))
		})
	})

	Describe("WithPoststopHooks", func() {
		It("adds the hook to the runtime spec", func() {
			returnedBundle := initialBundle.WithPoststopHooks(specs.Hook{
				Path: "foo",
				Args: []string{"bar", "baz"},
			})

			Expect(returnedBundle.PoststopHooks()).To(Equal([]specs.Hook{{
				Path: "foo",
				Args: []string{"bar", "baz"},
			}}))
		})
	})

	Describe("Mounts", func() {
		BeforeEach(func() {
			initialBundle = initialBundle.WithMounts(
				specs.Mount{
					Type:        "apple_fs",
					Source:      "iDevice",
					Destination: "/apple",
					Options: []string{
						"healthy",
						"shiny",
					},
				})
		})

		Describe("WithMounts", func() {
			BeforeEach(func() {
				returnedBundle = initialBundle.WithMounts(
					specs.Mount{
						Type:        "banana_fs",
						Source:      "banana_device",
						Destination: "/banana",
						Options: []string{
							"yellow",
							"fresh",
						},
					})
			})

			It("returns a bundle with the mounts appended to the spec", func() {
				Expect(returnedBundle.Mounts()[0]).To(Equal(
					specs.Mount{
						Destination: "/apple",
						Type:        "apple_fs",
						Source:      "iDevice",
						Options:     []string{"healthy", "shiny"},
					}))

				Expect(returnedBundle.Mounts()[1]).To(Equal(
					specs.Mount{
						Destination: "/banana",
						Type:        "banana_fs",
						Source:      "banana_device",
						Options:     []string{"yellow", "fresh"},
					},
				))
			})

			It("does not modify the original bundle", func() {
				Expect(returnedBundle).NotTo(Equal(initialBundle))
				Expect(initialBundle.Mounts()).To(HaveLen(1))
			})
		})

		Describe("WithPrependedMounts", func() {
			BeforeEach(func() {
				returnedBundle = initialBundle.WithPrependedMounts(
					specs.Mount{
						Type:        "banana_fs",
						Source:      "banana_device",
						Destination: "/banana",
						Options: []string{
							"yellow",
							"fresh",
						},
					})
			})

			It("returns a bundle with the mounts prepended to the spec", func() {
				Expect(returnedBundle.Mounts()[0]).To(Equal(
					specs.Mount{
						Destination: "/banana",
						Type:        "banana_fs",
						Source:      "banana_device",
						Options:     []string{"yellow", "fresh"},
					},
				))

				Expect(returnedBundle.Mounts()[1]).To(Equal(
					specs.Mount{
						Destination: "/apple",
						Type:        "apple_fs",
						Source:      "iDevice",
						Options:     []string{"healthy", "shiny"},
					}))
			})

			It("does not modify the original bundle", func() {
				Expect(returnedBundle).NotTo(Equal(initialBundle))
				Expect(initialBundle.Mounts()).To(HaveLen(1))
			})
		})
	})

	Describe("WithResources", func() {
		var t = true

		BeforeEach(func() {
			returnedBundle = initialBundle.WithResources(&specs.LinuxResources{DisableOOMKiller: &t})
		})

		It("returns a bundle with the resources added to the runtime spec", func() {
			Expect(returnedBundle.Resources()).To(Equal(&specs.LinuxResources{DisableOOMKiller: &t}))
		})

		It("does not modify the original bundle", func() {
			Expect(returnedBundle).NotTo(Equal(initialBundle))
			Expect(initialBundle.Resources()).To(BeNil())
		})
	})

	Describe("WithCPUShares", func() {
		var shares uint64 = 10

		BeforeEach(func() {
			returnedBundle = initialBundle.WithCPUShares(specs.LinuxCPU{Shares: &shares})
		})

		It("returns a bundle with the cpu shares added to the runtime spec", func() {
			Expect(returnedBundle.Resources().CPU).To(Equal(&specs.LinuxCPU{Shares: &shares}))
		})
	})

	Describe("WithBlockIO", func() {
		var blockIOWeight uint16 = 200

		BeforeEach(func() {
			returnedBundle = initialBundle.WithBlockIO(specs.LinuxBlockIO{Weight: &blockIOWeight})
		})

		It("returns a bundle with the block IO weight added to the runtime spec", func() {
			Expect(returnedBundle.Resources().BlockIO.Weight).To(Equal(&blockIOWeight))
		})
	})

	Describe("WithPidLimit", func() {
		var pidLimit int64 = 10

		BeforeEach(func() {
			returnedBundle = initialBundle.WithPidLimit(specs.LinuxPids{Limit: pidLimit})
		})

		It("returns a bundle with the pid limit added to the runtime spec", func() {
			Expect(returnedBundle.Resources().Pids).To(Equal(&specs.LinuxPids{Limit: pidLimit}))
		})
	})

	Describe("WithNamespace", func() {
		It("does not change any namespaces other than the one with the given type", func() {
			colin := specs.LinuxNamespace{Type: "colin", Path: ""}
			potato := specs.LinuxNamespace{Type: "potato", Path: "pan"}

			initialBundle = initialBundle.WithNamespace(colin)
			returnedBundle = initialBundle.WithNamespace(potato)
			Expect(returnedBundle.Namespaces()).To(ConsistOf(colin, potato))
		})

		Context("when the namespace isnt already in the spec", func() {
			It("adds the namespace", func() {
				ns := specs.LinuxNamespace{Type: "potato", Path: "pan"}
				returnedBundle = initialBundle.WithNamespace(ns)
				Expect(returnedBundle.Namespaces()).To(ConsistOf(ns))
			})
		})

		Context("when the namespace is already in the spec", func() {
			It("overrides the namespace", func() {
				initialBundle = initialBundle.WithNamespace(specs.LinuxNamespace{Type: "potato", Path: "should-be-overridden"})
				ns := specs.LinuxNamespace{Type: "potato", Path: "pan"}
				returnedBundle = initialBundle.WithNamespace(ns)
				Expect(returnedBundle.Namespaces()).To(ConsistOf(ns))
			})
		})
	})

	Describe("WithNamespaces", func() {
		BeforeEach(func() {
			returnedBundle = initialBundle.WithNamespaces(specs.LinuxNamespace{Type: specs.NetworkNamespace})
		})

		It("returns a bundle with the namespaces added to the runtime spec", func() {
			Expect(returnedBundle.Namespaces()).To(ConsistOf(specs.LinuxNamespace{Type: specs.NetworkNamespace}))
		})

		Context("when the spec already contains namespaces", func() {
			It("replaces them", func() {
				overridenBundle := returnedBundle.WithNamespaces(specs.LinuxNamespace{Type: "mynamespace"})
				Expect(overridenBundle.Namespaces()).To(ConsistOf(specs.LinuxNamespace{Type: "mynamespace"}))
			})
		})
	})

	Describe("WithUIDMappings", func() {
		It("returns a bundle with the provided uid mappings added to the runtime spec", func() {
			uidMappings := []specs.LinuxIDMapping{
				{
					HostID:      40000,
					ContainerID: 0,
					Size:        1,
				},
				{
					HostID:      1,
					ContainerID: 1,
					Size:        39999,
				},
			}
			returnedBundle := initialBundle.WithUIDMappings(uidMappings...)

			Expect(returnedBundle.UIDMappings()).To(Equal(uidMappings))
		})
	})

	Describe("WithGIDMappings", func() {
		It("returns a bundle with the provided gid mappings added to the runtime spec", func() {
			gidMappings := []specs.LinuxIDMapping{
				{
					HostID:      40000,
					ContainerID: 0,
					Size:        1,
				},
				{
					HostID:      1,
					ContainerID: 1,
					Size:        39999,
				},
			}
			returnedBundle := initialBundle.WithGIDMappings(gidMappings...)

			Expect(returnedBundle.GIDMappings()).To(Equal(gidMappings))
		})
	})

	Describe("WithDevices", func() {
		BeforeEach(func() {
			returnedBundle = initialBundle.WithDevices(specs.LinuxDevice{Path: "test/path"})
		})

		It("returns a bundle with the namespaces added to the runtime spec", func() {
			Expect(returnedBundle.Spec.Linux.Devices).To(ConsistOf(specs.LinuxDevice{Path: "test/path"}))
		})

		Context("when the spec already contains namespaces", func() {
			It("replaces them", func() {
				overridenBundle := returnedBundle.WithDevices(specs.LinuxDevice{Path: "new-device"})
				Expect(overridenBundle.Devices()).To(ConsistOf(specs.LinuxDevice{Path: "new-device"}))
			})
		})
	})

	Describe("NamespaceSlice", func() {
		Context("when the namespace isnt already in the slice", func() {
			It("adds the namespace", func() {
				ns := specs.LinuxNamespace{Type: "potato", Path: "pan"}
				nsl := goci.NamespaceSlice{}
				nsl = nsl.Set(ns)
				Expect(nsl).To(ConsistOf(ns))
			})
		})

		Context("when the namespace is already in the slice", func() {
			It("overrides the namespace", func() {
				ns := specs.LinuxNamespace{Type: "potato", Path: "pan"}
				nsl := goci.NamespaceSlice{specs.LinuxNamespace{Type: "potato", Path: "chips"}}
				nsl = nsl.Set(ns)
				Expect(nsl).To(ConsistOf(ns))
			})
		})
	})

	Describe("WithMaskedPaths", func() {
		It("sets the MaskedPaths in the bundle", func() {
			returnedBundle := initialBundle.WithMaskedPaths([]string{"path1", "path2"})
			paths := returnedBundle.MaskedPaths()
			Expect(len(paths)).To(Equal(2))
			Expect(paths[0]).To(Equal("path1"))
			Expect(paths[1]).To(Equal("path2"))
		})
	})
})
