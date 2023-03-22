package users_test

import (
	"code.cloudfoundry.org/guardian/rundmc/users"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LookupUser", func() {
	Context("when we try to get the UID, GID and HOME of a username", func() {
		Context("when we try to get the UID, GID and HOME of the empty string", func() {
			It("returns the default UID, GID and HOME", func() {
				user, err := users.LookupUser("", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(users.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(users.DefaultGID))
				Expect(user.Home).To(Equal(users.DefaultHome))
			})
		})
		Context("when a user is defined", func() {
			It("returns the HOME dir for the user", func() {
				user, err := users.LookupUser("", "vcap")
				Expect(err).ToNot(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(users.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(users.DefaultGID))
				Expect(user.Home).To(Equal("C:\\Users\\vcap"))
			})
		})
	})
})
