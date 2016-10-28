package storepath_test

import (
	"os/user"

	"code.cloudfoundry.org/grootfs/commands/storepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UserBased", func() {
	It("appends the current user id to the path", func() {
		currentUser, err := user.Current()
		Expect(err).NotTo(HaveOccurred())

		Expect(storepath.UserBased("/var/hello")).To(Equal("/var/hello/" + currentUser.Uid))
	})
})
