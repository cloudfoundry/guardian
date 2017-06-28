package runrunc_test

import (
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LookupUser", func() {
	Context("when we try to get the UID, GID and HOME of a username", func() {
		Context("when we try to get the UID, GID and HOME of the empty string", func() {
			It("returns the default UID, GID and HOME", func() {
				user, err := runrunc.LookupUser("", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(runrunc.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(runrunc.DefaultGID))
				Expect(user.Home).To(Equal(runrunc.DefaultHome))
			})
		})
		Context("when a user is defined", func() {
			It("returns the HOME dir for the user", func() {
				user, err := runrunc.LookupUser("", "vcap")
				Expect(err).ToNot(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(runrunc.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(runrunc.DefaultGID))
				Expect(user.Home).To(Equal("C:\\Users\\vcap"))
			})
		})
	})
})
