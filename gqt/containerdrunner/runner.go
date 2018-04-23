package containerdrunner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/burntsushi/toml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type Binaries struct {
	Dir        string
	Containerd string
	Ctr        string
}

type Config struct {
	Root      string      `toml:"root"`
	State     string      `toml:"state"`
	Subreaper bool        `toml:"subreaper"`
	OomScore  int         `toml:"oom_score"`
	GRPC      GRPCConfig  `toml:"grpc"`
	Debug     DebugConfig `toml:"debug"`

	BinariesDir   string
	ContainerdBin string
	CtrBin        string
	RunDir        string
}

type GRPCConfig struct {
	Address string `toml:"address"`
}

type DebugConfig struct {
	Address string `toml:"address"`
	Level   string `toml:"level"`
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
			Level:   "info",
		},
	}
}

// TODO: Get rid of NewDefaultSession
func NewSession(runDir string, bins Binaries, config Config) *gexec.Session {
	configFile, err := os.OpenFile(filepath.Join(runDir, "containerd.toml"), os.O_TRUNC|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
	Expect(toml.NewEncoder(configFile).Encode(&config)).To(Succeed())
	Expect(configFile.Close()).To(Succeed())

	cmd := exec.Command(bins.Containerd, "--config", configFile.Name())
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s", fmt.Sprintf("%s:%s", os.Getenv("PATH"), bins.Dir)))
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

func NewDefaultSession(config Config) *gexec.Session {
	configFile, err := os.OpenFile(filepath.Join(config.RunDir, "containerd.toml"), os.O_TRUNC|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
	Expect(toml.NewEncoder(configFile).Encode(&config)).To(Succeed())
	Expect(configFile.Close()).To(Succeed())

	cmd := exec.Command(config.ContainerdBin, "--config", configFile.Name())
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s", fmt.Sprintf("%s:%s", os.Getenv("PATH"), config.BinariesDir)))
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}
