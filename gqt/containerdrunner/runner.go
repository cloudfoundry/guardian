package containerdrunner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/burntsushi/toml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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
	UID     int    `toml:"uid"`
	GID     int    `toml:"gid"`
}

type DebugConfig struct {
	Address string `toml:"address"`
	Level   string `toml:"level"`
	UID     int    `toml:"uid"`
	GID     int    `toml:"gid"`
}

type Plugins struct {
	Linux Linux `toml:"linux"`
}

type Linux struct {
	ShimDebug   bool   `toml:"shim_debug"`
	RuntimeRoot string `toml:"runtime_root"`
}

func ContainerdConfig(containerdDataDir string) Config {
	return Config{
		Root:      filepath.Join(containerdDataDir, "root"),
		State:     filepath.Join(containerdDataDir, "state"),
		Subreaper: true,
		OomScore:  -999,
		GRPC: GRPCConfig{
			Address: filepath.Join(containerdDataDir, "containerd.sock"),
			UID:     5000,
			GID:     5000,
		},
		Debug: DebugConfig{
			Address: filepath.Join(containerdDataDir, "debug.sock"),
			Level:   "debug",
			UID:     5000,
			GID:     5000,
		},
		DisabledPlugins: []string{
			"aufs",
			"overlayfs",
			"zfs",
			"walking",
			"scheduler",
			"content-service",
			"diff-service",
			"images-service",
			"namespaces-service",
			"snapshots-service",
			"content",
			"diff",
			"events",
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
				ShimDebug:   true,
				RuntimeRoot: filepath.Join(containerdDataDir, "runtime_root"),
			},
		},
	}
}

func NewSession(runDir string, config Config) *gexec.Session {
	configFile, err := os.OpenFile(filepath.Join(runDir, "containerd.toml"), os.O_TRUNC|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
	Expect(toml.NewEncoder(configFile).Encode(&config)).To(Succeed())
	Expect(configFile.Close()).To(Succeed())

	Expect(os.Chown(configFile.Name(), 5000, 5000)).To(Succeed())

	cmd := exec.Command("containerd", "--config", configFile.Name())
	unprivilegedUser := &syscall.Credential{Uid: uint32(5000), Gid: uint32(5000)}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: unprivilegedUser}
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s", fmt.Sprintf("%s:%s", os.Getenv("PATH"), "/usr/local/bin")))
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}
