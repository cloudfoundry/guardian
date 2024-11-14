package containerdrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type Config struct {
	Version         int         `toml:"version"`
	Root            string      `toml:"root"`
	State           string      `toml:"state"`
	Subreaper       bool        `toml:"subreaper"`
	OomScore        int         `toml:"oom_score"`
	GRPC            GRPCConfig  `toml:"grpc"`
	Debug           DebugConfig `toml:"debug"`
	DisabledPlugins []string    `toml:"disabled_plugins"`
	Plugins         Plugins     `toml:"plugins"`

	RunDir string
}

type GRPCConfig struct {
	Address string `toml:"address"`
}

type DebugConfig struct {
	Address string `toml:"address"`
	Level   string `toml:"level"`
}

type Plugins struct {
	IoContainerdGrpcV1Linux IoContainerdGrpcV1Linux `toml:"io.containerd.runtime.v1.linux"`
	IoContainerdGrpcV1Cri   IoContainerdGrpcV1Cri   `toml:"io.containerd.grpc.v1.cri"`
}
type IoContainerdGrpcV1Cri struct {
	IoContainerdGrpcV1CriContainerd IoContainerdGrpcV1CriContainerd `toml:"containerd"`
}

type IoContainerdGrpcV1CriContainerd struct {
	ContainerdRuntimes ContainerdRuntimes `toml:"runtimes"`
}

type ContainerdRuntimes struct {
	RuntimesRunc RuntimesRunc `toml:"runc"`
}

type RuntimesRunc struct {
	RuntimeType string `toml:"runtime_type"`
}

type IoContainerdGrpcV1Linux struct {
	ShimDebug bool `toml:"shim_debug"`
}

func ContainerdConfig(containerdDataDir string) Config {
	return Config{
		Version:   2,
		Root:      filepath.Join(containerdDataDir, "root"),
		State:     filepath.Join(containerdDataDir, "state"),
		Subreaper: true,
		OomScore:  -999,
		GRPC: GRPCConfig{
			Address: filepath.Join(containerdDataDir, "containerd.sock"),
		},
		Debug: DebugConfig{
			Address: filepath.Join(containerdDataDir, "debug.sock"),
			Level:   "debug",
		},
		DisabledPlugins: []string{
			"io.containerd.snapshotter.v1.aufs",
			"io.containerd.snapshotter.v1.devmapper",
			"io.containerd.snapshotter.v1.overlayfs",
			"io.containerd.snapshotter.v1.zfs",
			"io.containerd.differ.v1.walking",
			"io.containerd.gc.v1.scheduler",
			"io.containerd.service.v1.diff-service",
			"io.containerd.service.v1.images-service",
			"io.containerd.service.v1.namespaces-service",
			"io.containerd.service.v1.snapshots-service",
			"io.containerd.service.v1.leases-service",
			"io.containerd.grpc.v1.diff",
			"io.containerd.grpc.v1.healthcheck",
			"io.containerd.grpc.v1.images",
			"io.containerd.grpc.v1.snapshots",
			"io.containerd.grpc.v1.version",
			"io.containerd.grpc.v1.cri",
			"io.containerd.grpc.v1.leases",
			"io.containerd.internal.v1.restart",
		},
		Plugins: Plugins{
			IoContainerdGrpcV1Linux: IoContainerdGrpcV1Linux{
				ShimDebug: true,
			},
			IoContainerdGrpcV1Cri: IoContainerdGrpcV1Cri{
				IoContainerdGrpcV1CriContainerd{ContainerdRuntimes{RuntimesRunc{RuntimeType: "io.containerd.runc.v2"}}},
			},
		},
	}
}

func NewContainerdProcess(runDir string, config Config) *os.Process {
	configFile, err := os.OpenFile(filepath.Join(runDir, "containerd.toml"), os.O_TRUNC|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
	Expect(toml.NewEncoder(configFile).Encode(&config)).To(Succeed())
	Expect(configFile.Close()).To(Succeed())

	cmd := exec.Command("containerd", "--config", configFile.Name())
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s", fmt.Sprintf("%s:%s", os.Getenv("PATH"), "/usr/local/bin")))
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Start()).To(Succeed())
	Eventually(func() error { return ping(config) }, 10*time.Second, time.Second).Should(Succeed(), "containerd is taking too long to become available")
	return cmd.Process
}

func ping(config Config) error {
	client, err := client.New(config.GRPC.Address, containerd.WithDefaultRuntime(plugins.RuntimeLinuxV1))
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Containers(namespaces.WithNamespace(context.Background(), "garden"))
	return err
}
