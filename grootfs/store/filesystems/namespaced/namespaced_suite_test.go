package namespaced_test

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func init() {
	fmt.Fprintf(os.Stderr, "uid: %d, gid: %d\n", os.Getuid(), os.Getgid())
	if reexec.Init() {
		os.Exit(0)
	}

	reexec.Register("no-root", func() {
	})
}

func TestNamespaced(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		Expect(rerunTestsAsUser(1000)).To(Succeed())

		return nil
	}, func(_ []byte) {
	})

	RunSpecs(t, "Namespaced Suite")
}

func rerunTestsAsUser(uid uint32) error {
	if os.Getuid() == int(uid) {
		return nil
	}

	cmd := reexec.Command("no-root")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: uid},
	}

	sess, err := gexec.Start(cmd, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
	return nil
}
