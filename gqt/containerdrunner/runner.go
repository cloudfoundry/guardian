package containerdrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/plugin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Config struct {
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
	Linux Linux `toml:"linux"`
}

type Linux struct {
	ShimDebug bool `toml:"shim_debug"`
}

func ContainerdConfig(containerdDataDir string) Config {
	return Config{
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
			"aufs",
			"devmapper",
			"overlayfs",
			"zfs",
			"walking",
			"scheduler",
			"diff-service",
			"images-service",
			"namespaces-service",
			"snapshots-service",
			"diff",
			"healthcheck",
			"images",
			"namespaces",
			"snapshots",
			"version",
			"cri",
			"leases",
			"leases-service",
			"restart",
		},
		Plugins: Plugins{
			Linux: Linux{
				ShimDebug: true,
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
	client, err := containerd.New(config.GRPC.Address, containerd.WithDefaultRuntime(plugin.RuntimeLinuxV1))
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Containers(namespaces.WithNamespace(context.Background(), "garden"))
	return err
}
