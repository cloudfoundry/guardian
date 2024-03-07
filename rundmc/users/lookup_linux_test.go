package users_test

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/users"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LookupUser", func() {
	Context("when we try to get the UID, GID and HOME of a username", func() {
		var (
			rootFsPath     string
			uidOfTheBeast  = 666
			gidOfTheBeast  = 777
			homeOfTheBeast = "/home/fieryunderworld"
		)

		createPasswdFile := func() error {
			return os.WriteFile(filepath.Join(rootFsPath, "etc", "passwd"), []byte(
				`_lda:*:211:211:Local Delivery Agent:/var/empty:/usr/bin/false
_cvmsroot:*:212:212:CVMS Root:/var/empty:/usr/bin/false
_usbmuxd:*:213:213:iPhone OS Device Helper:/var/db/lockdown:/usr/bin/false
devil:*:666:777:Beelzebub:/home/fieryunderworld:/usr/bin/false
_dovecot:*:214:6:Dovecot Administrator:/var/empty:/usr/bin/false
vcap:*:1000:1000:VCAP:/home/vcap/:/bin/sh`,
			), 0777)
		}

		createGroupFile := func() error {
			return os.WriteFile(filepath.Join(rootFsPath, "etc", "group"), []byte(
				`root:x:0:
daemon:x:1:
bin:x:2:
vcap:x:1000:
another:x:4:vcap`,
			), 0777)
		}

		BeforeEach(func() {
			var err error
			rootFsPath, err = os.MkdirTemp("", "passwdtestdir")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(rootFsPath, "etc"), 0777)).To(Succeed())
			Expect(createGroupFile()).To(Succeed())
			Expect(createPasswdFile()).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(rootFsPath)).To(Succeed())
		})

		DescribeTable("user / group combinations", func(username string, expectedUid, expectedGid int, expectedHome string) {
			user, err := users.LookupUser(rootFsPath, username)
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(expectedUid))
			Expect(user.Gid).To(BeEquivalentTo(expectedGid))
			Expect(user.Home).To(Equal(expectedHome))
		},

			Entry("empty username", "", users.DefaultUID, users.DefaultGID, users.DefaultHome),
			Entry("username from /etc/passwd", "devil", uidOfTheBeast, gidOfTheBeast, homeOfTheBeast),
			Entry("username from /etc/passwd and groupname from /etc/group", "devil:daemon", uidOfTheBeast, 1, homeOfTheBeast),
			Entry("username from /etc/passwd and given groupid", "devil:123", uidOfTheBeast, 123, homeOfTheBeast),
			Entry("given userid and groupname from /etc/group", "666:vcap", uidOfTheBeast, 1000, homeOfTheBeast),
			Entry("given userid and groupid", "123:456", 123, 456, "/"),
			Entry("given userid that exists in /etc/passwd", "666", uidOfTheBeast, gidOfTheBeast, homeOfTheBeast),
		)

		Context("when /etc/passwd exists with no matching users", func() {
			It("returns an error", func() {
				_, err := users.LookupUser(rootFsPath, "unknownUser")
				Expect(err).To(MatchError(ContainSubstring("unable to find")))
			})
		})

		Context("when /etc/passwd exists but cannot be parsed", func() {
			BeforeEach(func() {
				senselessContents := []byte(
					`lorem ipsum dollar sit amet
					unix at the portal
					body type by letroset
					here at the epoch
					let us forget...`,
				)
				passwdPath := filepath.Join(rootFsPath, "etc", "passwd")
				Expect(os.WriteFile(passwdPath, senselessContents, 0777)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := users.LookupUser(rootFsPath, "devil")
				Expect(err).To(MatchError(ContainSubstring("unable to find")))
			})
		})

		Context("when /etc/passwd does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(rootFsPath, "etc", "passwd"))).To(Succeed())
			})

			It("returns the default UID, GID and HOME when user 'root' is requested", func() {
				user, err := users.LookupUser(rootFsPath, "root")
				Expect(err).NotTo(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(users.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(users.DefaultGID))
				Expect(user.Home).To(Equal(users.DefaultHome))
			})

			It("errors when a user other than root is requested", func() {
				_, err := users.LookupUser(rootFsPath, "nobody")
				Expect(err).To(MatchError(ContainSubstring("unable to find user nobody")))
			})
		})

		Context("when /etc/group does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(rootFsPath, "etc", "group"))).To(Succeed())
			})

			It("tolerates a numeric gid", func() {
				_, err := users.LookupUser(rootFsPath, "123:456")
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors when a group name is requested", func() {
				_, err := users.LookupUser(rootFsPath, "123:devil")
				Expect(err).To(MatchError(ContainSubstring("unable to find group devil")))
			})
		})

		Context("secondary groups", func() {
			It("has vcap in a secondary group", func() {
				user, err := users.LookupUser(rootFsPath, "vcap")
				Expect(err).NotTo(HaveOccurred())
				Expect(user.Gid).To(Equal(1000))
				Expect(user.Sgids).To(HaveLen(1))
				Expect(user.Sgids[0]).To(Equal(4))
			})
		})
	})
})
