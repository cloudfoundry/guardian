package manager_test

import (
	"os"
	"os/user"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	GrootUID uint32
	GrootGID uint32
)

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		GrootUser, err := user.Lookup(os.Getenv("GROOTFS_USER"))
		Expect(err).NotTo(HaveOccurred())

		grootUID, err := strconv.ParseUint(GrootUser.Uid, 10, 32)
		Expect(err).NotTo(HaveOccurred())
		GrootUID = uint32(grootUID)

		grootGID, err := strconv.ParseUint(GrootUser.Gid, 10, 32)
		Expect(err).NotTo(HaveOccurred())
		GrootGID = uint32(grootGID)
	})

	RunSpecs(t, "Manager Suite")
}
