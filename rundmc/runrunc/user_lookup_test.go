package runrunc_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("LookupUser", func() {
	Context("when we try to get the UID, GID and HOME of a username", func() {
		var (
			rootFsPath string
		)

		BeforeEach(func() {
			var err error
			rootFsPath, err = ioutil.TempDir("", "passwdtestdir")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(rootFsPath, "etc"), 0777)).To(Succeed())
		})

		Context("when we try to get the UID, GID and HOME of the empty string", func() {
			It("returns the default UID, GID and HOME", func() {
				rootFsPath = "some path"
				user, err := runrunc.LookupUser(rootFsPath, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(runrunc.DefaultUID))
				Expect(user.Gid).To(BeEquivalentTo(runrunc.DefaultGID))
				Expect(user.Home).To(Equal(runrunc.DefaultHome))
			})
		})

		Context("when /etc/passwd exists with one matching user", func() {
			const ()
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(rootFsPath, "etc", "passwd"), []byte(
					`_lda:*:211:211:Local Delivery Agent:/var/empty:/usr/bin/false
_cvmsroot:*:212:212:CVMS Root:/var/empty:/usr/bin/false
_usbmuxd:*:213:213:iPhone OS Device Helper:/var/db/lockdown:/usr/bin/false
devil:*:666:777:Beelzebub:/home/fieryunderworld:/usr/bin/false
_dovecot:*:214:6:Dovecot Administrator:/var/empty:/usr/bin/false`,
				), 0777)).To(Succeed())
			})

			It("gets the user ID from /etc/passwd", func() {
				user, err := runrunc.LookupUser(rootFsPath, "devil")
				Expect(err).ToNot(HaveOccurred())
				Expect(user.Uid).To(BeEquivalentTo(666))             // the UID of the beast
				Expect(user.Gid).To(BeEquivalentTo(777))             // the GID of the beast
				Expect(user.Home).To(Equal("/home/fieryunderworld")) // the Home of the beast
			})
		})

		Context("when /etc/passwd exists with no matching users", func() {
			It("returns an error", func() {
				Expect(ioutil.WriteFile(filepath.Join(rootFsPath, "etc", "passwd"), []byte{}, 0777)).To(Succeed())

				_, err := runrunc.LookupUser(rootFsPath, "unknownUser")
				Expect(err).To(MatchError(ContainSubstring("unable to find")))
			})
		})

		DescribeTable("when /etc/passwd cannot be parsed", func(breakEtcPasswd func()) {
			breakEtcPasswd()
			_, err := runrunc.LookupUser(rootFsPath, "devil")
			Expect(err).To(MatchError(ContainSubstring("unable to find")))
		},
			Entry("because it doesn't exist", func() {}),
			Entry("because the contents makes no sense", func() {
				senselessContents := []byte(
					`lorem ipsum dollar sit amet
					unix at the portal
					body type by letroset
					here at the epoch
					let us forget...`,
				)
				passwdPath := filepath.Join(rootFsPath, "etc", "passwd")
				Expect(ioutil.WriteFile(passwdPath, senselessContents, 0777)).To(Succeed())
			}))
	})
})
