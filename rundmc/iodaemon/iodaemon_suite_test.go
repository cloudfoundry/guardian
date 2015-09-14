package iodaemon_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var iodaemonBinPath string
var testPrintSignalBinPath string

var tmpdir string
var socketPath string

type CompiledAssets struct {
	IoDaemonBinPath string
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	assets := CompiledAssets{}
	assets.IoDaemonBinPath, err = gexec.Build("github.com/cloudfoundry-incubator/guardian/rundmc/iodaemon/cmd/iodaemon", "-race")
	Expect(err).ToNot(HaveOccurred())

	marshalledAssets, err := json.Marshal(assets)
	Expect(err).ToNot(HaveOccurred())
	return marshalledAssets
}, func(marshalledAssets []byte) {
	assets := CompiledAssets{}
	err := json.Unmarshal(marshalledAssets, &assets)
	Expect(err).ToNot(HaveOccurred())
	iodaemonBinPath = assets.IoDaemonBinPath
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	var err error

	tmpdir, err = ioutil.TempDir("", "socket-dir")
	Expect(err).ToNot(HaveOccurred())

	socketPath = filepath.Join(tmpdir, "iodaemon.sock")
})

var _ = AfterEach(func() {
	os.RemoveAll(tmpdir)
})

func TestIodaemon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Iodaemon Suite")
}
