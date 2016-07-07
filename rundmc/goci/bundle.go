package goci

import "github.com/opencontainers/runtime-spec/specs-go"

// Bndl represents an in-memory OCI bundle
type Bndl struct {
	Spec specs.Spec
}

// Bundle creates a Bndl
func Bundle() Bndl {
	return Bndl{
		Spec: specs.Spec{
			Version: "0.2.0",
		},
	}
}

var (
	NetworkNamespace = specs.Namespace{Type: specs.NetworkNamespace}
	UserNamespace    = specs.Namespace{Type: specs.UserNamespace}
	PIDNamespace     = specs.Namespace{Type: specs.PIDNamespace}
	IPCNamespace     = specs.Namespace{Type: specs.IPCNamespace}
	UTSNamespace     = specs.Namespace{Type: specs.UTSNamespace}
	MountNamespace   = specs.Namespace{Type: specs.MountNamespace}
)

// WithProcess returns a bundle with the process replaced with the given Process. The original bundle is not modified.
func (b Bndl) WithProcess(process specs.Process) Bndl {
	b.Spec.Process = process
	return b
}

func (b Bndl) Hostname() string {
	return b.Spec.Hostname
}

func (b Bndl) WithHostname(hostname string) Bndl {
	b.Spec.Hostname = hostname
	return b
}

func (b Bndl) Process() specs.Process {
	return b.Spec.Process
}

func (b Bndl) WithRootFS(absolutePath string) Bndl {
	b.Spec.Root = specs.Root{Path: absolutePath}
	return b
}

// GetRootfsPath returns the path to the rootfs of this bundle. Nothing is modified
func (b Bndl) RootFS() string {
	return b.Spec.Root.Path
}

// WithResources returns a bundle with the resources replaced with the given Resources. The original bundle is not modified.
func (b Bndl) WithResources(resources *specs.Resources) Bndl {
	b.Spec.Linux.Resources = resources
	return b
}

func (b Bndl) Resources() *specs.Resources {
	return b.Spec.Linux.Resources
}

func (b Bndl) WithCPUShares(shares specs.CPU) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.Resources{}
	}

	resources.CPU = &shares
	b.Spec.Linux.Resources = resources

	return b
}

func (b Bndl) WithMemoryLimit(limit specs.Memory) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.Resources{}
	}

	resources.Memory = &limit
	b.Spec.Linux.Resources = resources

	return b
}

// WithNamespace returns a bundle with the given namespace in the list of namespaces. The bundle is not modified, but any
// existing namespace of this type will be replaced.
func (b Bndl) WithNamespace(ns specs.Namespace) Bndl {
	slice := NamespaceSlice(b.Spec.Linux.Namespaces)
	b.Spec.Linux.Namespaces = []specs.Namespace(slice.Set(ns))
	return b
}

func (b Bndl) Namespaces() []specs.Namespace {
	return b.Spec.Linux.Namespaces
}

func (b Bndl) WithUIDMappings(mappings ...specs.IDMapping) Bndl {
	b.Spec.Linux.UIDMappings = mappings
	return b
}

func (b Bndl) UIDMappings() []specs.IDMapping {
	return b.Spec.Linux.UIDMappings
}

func (b Bndl) WithGIDMappings(mappings ...specs.IDMapping) Bndl {
	b.Spec.Linux.GIDMappings = mappings
	return b
}

func (b Bndl) GIDMappings() []specs.IDMapping {
	return b.Spec.Linux.GIDMappings
}

func (b Bndl) WithPrestartHooks(hook ...specs.Hook) Bndl {
	b.Spec.Hooks.Prestart = hook
	return b
}

func (b Bndl) PrestartHooks() []specs.Hook {
	return b.Spec.Hooks.Prestart
}

func (b Bndl) WithPoststopHooks(hook ...specs.Hook) Bndl {
	b.Spec.Hooks.Poststop = hook
	return b
}

func (b Bndl) PoststopHooks() []specs.Hook {
	return b.Spec.Hooks.Poststop
}

// WithNamespaces returns a bundle with the given namespaces. The original bundle is not modified, but the original
// set of namespaces is replaced in the returned bundle.
func (b Bndl) WithNamespaces(namespaces ...specs.Namespace) Bndl {
	b.Spec.Linux.Namespaces = namespaces
	return b
}

// WithDevices returns a bundle with the given devices added. The original bundle is not modified.
func (b Bndl) WithDevices(devices ...specs.Device) Bndl {
	b.Spec.Linux.Devices = devices
	return b
}

func (b Bndl) Devices() []specs.Device {
	return b.Spec.Linux.Devices
}

// WithCapabilities returns a bundle with the given capabilities added. The original bundle is not modified.
func (b Bndl) WithCapabilities(capabilities ...string) Bndl {
	b.Spec.Process.Capabilities = capabilities
	return b
}

func (b Bndl) Capabilities() []string {
	return b.Spec.Process.Capabilities
}

// WithMounts returns a bundle with the given mounts added. The original bundle is not modified.
func (b Bndl) WithMounts(mounts ...specs.Mount) Bndl {
	b.Spec.Mounts = append(b.Spec.Mounts, mounts...)
	return b
}

func (b Bndl) Mounts() []specs.Mount {
	return b.Spec.Mounts
}

func (b Bndl) WithMaskedPaths(maskedPaths []string) Bndl {
	b.Spec.Linux.MaskedPaths = maskedPaths
	return b
}

func (b Bndl) MaskedPaths() []string {
	return b.Spec.Linux.MaskedPaths
}

type NamespaceSlice []specs.Namespace

func (slice NamespaceSlice) Set(ns specs.Namespace) NamespaceSlice {
	for i, namespace := range slice {
		if namespace.Type == ns.Type {
			slice[i] = ns
			return slice
		}
	}

	return append(slice, ns)
}

// Process returns an OCI Process struct with the given args.
func Process(args ...string) specs.Process {
	return specs.Process{Args: args}
}
