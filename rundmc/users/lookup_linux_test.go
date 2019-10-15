package users_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/users"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LookupUser", func() {
	Context("when we try to get the UID, GID and HOME of a username", func() {
		var (
			rootFsPath string
		)

		createPasswdFile := func() error {
			return ioutil.WriteFile(filepath.Join(rootFsPath, "etc", "passwd"), []byte(
				`_lda:*:211:211:Local Delivery Agent:/var/empty:/usr/bin/false
_cvmsroot:*:212:212:CVMS Root:/var/empty:/usr/bin/false
_usbmuxd:*:213:213:iPhone OS Device Helper:/var/db/lockdown:/usr/bin/false
devil:*:666:777:Beelzebub:/home/fieryunderworld:/usr/bin/false
_dovecot:*:214:6:Dovecot Administrator:/var/empty:/usr/bin/false`,
			), 0777)
		}

		createGroupFile := func() error {
			return ioutil.WriteFile(filepath.Join(rootFsPath, "etc", "group"), []byte(
				`root:x:0:
daemon:x:1:
bin:x:2:
vcap:x:1000:`,
			), 0777)
		}

		BeforeEach(func() {
			var err error
			rootFsPath, err = ioutil.TempDir("", "passwdtestdir")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(rootFsPath, "etc"), 0777)).To(Succeed())
			Expect(createGroupFile()).To(Succeed())
			Expect(createPasswdFile()).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(rootFsPath)).To(Succeed())
		})

		It("returns the default UID, GID and HOME for an empty user string", func() {
			user, err := users.LookupUser(rootFsPath, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(users.DefaultUID))
			Expect(user.Gid).To(BeEquivalentTo(users.DefaultGID))
			Expect(user.Home).To(Equal(users.DefaultHome))
		})

		It("finds a match for a given user in /etc/passwd", func() {
			user, err := users.LookupUser(rootFsPath, "devil")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(666))             // the UID of the beast
			Expect(user.Gid).To(BeEquivalentTo(777))             // the GID of the beast
			Expect(user.Home).To(Equal("/home/fieryunderworld")) // the Home of the beast
		})

		It("looks up userid from /etc/passwd and groupid from /etc/group given username and groupname", func() {
			user, err := users.LookupUser(rootFsPath, "devil:daemon")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(666))
			Expect(user.Gid).To(BeEquivalentTo(1))
			Expect(user.Home).To(Equal("/home/fieryunderworld"))
		})

		It("looks up userid from /etc/passwd and returns groupid given username and groupid", func() {
			user, err := users.LookupUser(rootFsPath, "devil:123")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(666))
			Expect(user.Gid).To(BeEquivalentTo(123))
			Expect(user.Home).To(Equal("/home/fieryunderworld"))
		})

		It("returns userid and looks up groupid from /etc/group given userid and groupname", func() {
			user, err := users.LookupUser(rootFsPath, "666:vcap")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(666))
			Expect(user.Gid).To(BeEquivalentTo(1000))
			Expect(user.Home).To(Equal("/home/fieryunderworld"))
		})

		It("returns userid and groupid given userid and groupid", func() {
			user, err := users.LookupUser(rootFsPath, "123:456")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(123))
			Expect(user.Gid).To(BeEquivalentTo(456))
			Expect(user.Home).To(Equal("/"))
		})

		It("returns userid and groupid given userid contained in /etc/passwd", func() {
			user, err := users.LookupUser(rootFsPath, "666")
			Expect(err).ToNot(HaveOccurred())
			Expect(user.Uid).To(BeEquivalentTo(666))
			Expect(user.Gid).To(BeEquivalentTo(777))
			Expect(user.Home).To(Equal("/home/fieryunderworld"))
		})

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
				Expect(ioutil.WriteFile(passwdPath, senselessContents, 0777)).To(Succeed())
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

			It("returns the default UID, GID and HOME when user root is requested", func() {
				user, err := users.LookupUser(rootFsPath, "root")
				Expect(err).NotTo(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(users.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(users.DefaultGID))
				Expect(user.Home).To(Equal(users.DefaultHome))
			})

			It("returns the default UID, GID and HOME when user root:root is requested", func() {
				user, err := users.LookupUser(rootFsPath, "root:root")
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
	})
})
