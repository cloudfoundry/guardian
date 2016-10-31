package storepath_test

import (
	"os"
	"strconv"

	"code.cloudfoundry.org/grootfs/commands/storepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UserBased", func() {
	It("appends the current user id to the path", func() {
		userID := os.Getuid()
		Expect(storepath.UserBased("/var/hello")).To(Equal("/var/hello/" + strconv.Itoa(userID)))
	})
})
