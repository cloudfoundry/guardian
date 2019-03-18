package helpers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/guardian/gqt/runner"
	"github.com/burntsushi/toml"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func FindInGoPathBin(binary string) string {
	gopath, ok := os.LookupEnv("GOPATH")
	Expect(ok).To(BeTrue(), "GOPATH must be set")
	binPath := filepath.Join(gopath, "bin", binary)
	Expect(binPath).To(BeAnExistingFile(), fmt.Sprintf("%s does not exist", binPath))
	return binPath
}

func DefaultConfig(defaultTestRootFS string, binaries runner.Binaries) runner.GdnRunnerConfig {
	cfg := runner.DefaultGdnRunnerConfig(binaries)
	cfg.DefaultRootFS = defaultTestRootFS
	cfg.GdnBin = binaries.Gdn
	cfg.GrootBin = binaries.Groot
	cfg.Socket2meBin = binaries.Socket2me
	cfg.ExecRunnerBin = binaries.ExecRunner
	cfg.InitBin = binaries.Init
	cfg.TarBin = binaries.Tar
	cfg.NSTarBin = binaries.NSTar
	cfg.ImagePluginBin = binaries.Groot
	cfg.PrivilegedImagePluginBin = binaries.Groot

	return cfg
}

func GetGardenBinaries() runner.Binaries {
	gardenBinaries := runner.Binaries{
		Tar:           os.Getenv("GARDEN_TAR_PATH"),
		Gdn:           CompileGdn(),
		NetworkPlugin: goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_network_plugin"),
		ImagePlugin:   goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_image_plugin"),
		RuntimePlugin: goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_runtime_plugin"),
		NoopPlugin:    goCompile("code.cloudfoundry.org/guardian/gqt/cmd/noop_plugin"),
	}

	gardenBinaries.PrivilegedImagePlugin = gardenBinaries.ImagePlugin + "-priv"
	Expect(copyFile(gardenBinaries.ImagePlugin, gardenBinaries.PrivilegedImagePlugin)).To(Succeed())

	if runtime.GOOS == "linux" {
		gardenBinaries.ExecRunner = goCompile("code.cloudfoundry.org/guardian/cmd/dadoo")
		gardenBinaries.Socket2me = goCompile("code.cloudfoundry.org/guardian/cmd/socket2me")

		cmd := exec.Command("make")
		RunCommandInDir(cmd, "../rundmc/nstar")
		gardenBinaries.NSTar = "../rundmc/nstar/nstar"

		cmd = exec.Command("gcc", "-static", "-o", "init", "init.c", "ignore_sigchild.c")
		RunCommandInDir(cmd, "../cmd/init")
		gardenBinaries.Init = "../cmd/init/init"

		gardenBinaries.Groot = FindInGoPathBin("grootfs")
		gardenBinaries.Tardis = FindInGoPathBin("tardis")
	}

	return gardenBinaries
}

func CompileGdn(additionalCompileArgs ...string) string {
	defaultCompileArgs := []string{"-tags", "daemon"}
	compileArgs := append(defaultCompileArgs, additionalCompileArgs...)

	return goCompile("code.cloudfoundry.org/guardian/cmd/gdn", compileArgs...)
}

func goCompile(mainPackagePath string, buildArgs ...string) string {
	if os.Getenv("RACE_DETECTION") != "" {
		buildArgs = append(buildArgs, "-race")
	}
	bin, err := gexec.Build(mainPackagePath, buildArgs...)
	Expect(err).NotTo(HaveOccurred())
	return bin
}

func RunCommandInDir(cmd *exec.Cmd, workingDir string) string {
	cmd.Dir = workingDir
	cmdOutput, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Running command %#v failed: %v: %s", cmd, err, string(cmdOutput)))
	return string(cmdOutput)
}

func copyFile(srcPath, dstPath string) error {
	dirPath := filepath.Dir(dstPath)
	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return err
	}

	reader, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	writer, err := os.Create(dstPath)
	if err != nil {
		reader.Close()
		return err
	}

	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		reader.Close()
		return err
	}

	writer.Close()
	reader.Close()

	return os.Chmod(writer.Name(), 0777)
}

func JsonMarshal(v interface{}) []byte {
	buf := bytes.NewBuffer([]byte{})
	Expect(toml.NewEncoder(buf).Encode(v)).To(Succeed())
	return buf.Bytes()
}

func JsonUnmarshal(data []byte, v interface{}) {
	Expect(toml.Unmarshal(data, v)).To(Succeed())
}

func ReadFile(path string) []byte {
	content, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return content
}
