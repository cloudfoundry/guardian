package runrunc_test

import (
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	depotfakes "github.com/cloudfoundry-incubator/guardian/rundmc/depot/fakes"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("UidGidGetter", func() {
	const (
		ContainerID string = "123"
	)
	Context("when we try to get the Uid and Gid of a username", func() {
		var (
			idgetter             runrunc.UidGidGetter
			fakeBundleGetter     *fakes.FakeBundleGetter
			logger               lager.Logger
			fakeBundleLoader     *depotfakes.FakeBundleLoader
			fakePasswdFileParser *fakes.FakePasswdFileParserObject
		)

		BeforeEach(func() {
			fakeBundleGetter = &fakes.FakeBundleGetter{}
			logger = lagertest.NewTestLogger("test-id-getter")
			fakeBundleLoader = &depotfakes.FakeBundleLoader{}
			fakePasswdFileParser = &fakes.FakePasswdFileParserObject{}
			idgetter = runrunc.UidGidGetter{
				BundleGetter:     fakeBundleGetter,
				Logger:           logger,
				BundleLoader:     fakeBundleLoader,
				PasswdFileParser: fakePasswdFileParser.ParsePasswdFile,
			}
		})

		Context("when we try to get the Uid and Gid of the empty string", func() {
			It("returns 0,0 for the root user", func() {
				graphPath := "graph-"
				fakeBundleGetter.GetBundleReturns(&goci.Bndl{Spec: specs.LinuxSpec{Spec: specs.Spec{Root: specs.Root{Path: graphPath}}}}, nil)
				resultUID, resultGID, err := idgetter.GetIDs(ContainerID, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(resultUID).To(Equal(uint32(0)))
				Expect(resultGID).To(Equal(uint32(0)))
			})
		})

		Context("when /etc/passwd exists with one matching user", func() {
			const (
				UID      uint32 = 666
				GID      uint32 = 777
				UserName string = "devil"
			)
			var (
				resultUID uint32
				resultGID uint32
				graphPath string
			)
			BeforeEach(func() {
				graphPath = "graph-"
				fakeBundleGetter.GetBundleReturns(&goci.Bndl{Spec: specs.LinuxSpec{Spec: specs.Spec{Root: specs.Root{Path: graphPath}}}}, nil)

				fakePasswdFileParser.ParsePasswdFileReturns([]user.User{
					user.User{Name: "notFound", Uid: int(4), Gid: int(7)},
					user.User{Name: UserName, Uid: int(UID), Gid: int(GID)},
				}, nil)

				var err error
				resultUID, resultGID, err = idgetter.GetIDs(ContainerID, UserName)
				Expect(err).ToNot(HaveOccurred())
			})

			It("delgates finding the bundle to the BundleGetter", func() {
				Expect(fakeBundleGetter.GetBundleCallCount()).To(Equal(1))
				locallogger, loader, id := fakeBundleGetter.GetBundleArgsForCall(0)
				Expect(locallogger).To(Equal(logger))
				Expect(loader).To(Equal(fakeBundleLoader))
				Expect(id).To(Equal(ContainerID))
			})

			It("delgates parsing to passwd file to PasswdFileParser", func() {
				expectedPasswdPath := filepath.Join(graphPath, "etc", "passwd")
				Expect(fakePasswdFileParser.ParsePasswdFileCallCount()).To(Equal(1))
				Expect(fakePasswdFileParser.ParsePasswdFileArgsForCall(0)).To(Equal(expectedPasswdPath))
			})

			It("gets the user ID from /etc/passwd", func() {
				Expect(resultUID).To(Equal(UID))
				Expect(resultGID).To(Equal(GID))
			})

		})

		Context("when /etc/passwd exists with no matching users", func() {
			It("returns an error", func() {
				graphPath := "graph-"
				fakeBundleGetter.GetBundleReturns(&goci.Bndl{Spec: specs.LinuxSpec{Spec: specs.Spec{Root: specs.Root{Path: graphPath}}}}, nil)

				_, _, err := idgetter.GetIDs(ContainerID, "unknownUser")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when there is a problem getting the bundle", func() {
			Context("because GetBundle returns an error", func() {
				It("it propagates the error", func() {
					fakeBundleGetter.GetBundleReturns(nil, fmt.Errorf("GetBundle error"))
					_, _, err := idgetter.GetIDs(ContainerID, "bob")
					Expect(err).To(MatchError("GetBundle error"))
				})

				Context("even when we ask for the default UID/GID", func() {
					It("it returns an error", func() {
						fakeBundleGetter.GetBundleReturns(nil, fmt.Errorf("GetBundle error"))
						_, _, err := idgetter.GetIDs(ContainerID, "")
						Expect(err).To(MatchError("GetBundle error"))
					})
				})

			})

			DescribeTable("because of a malformed bundle",
				func(breakGetBundle func(), username string) {
					breakGetBundle()
					_, _, err := idgetter.GetIDs(ContainerID, username)
					Expect(err).To(HaveOccurred())
				},

				Entry("which is nil", func() {
					fakeBundleGetter.GetBundleReturns(nil, nil)
				}, "bob"),

				Entry("which contains an empty rootfs path", func() {
					fakeBundleGetter.GetBundleReturns(&goci.Bndl{}, nil)
				}, "bob"),

				Entry("which is nil (even when asking for the default user)", func() {
					fakeBundleGetter.GetBundleReturns(nil, nil)
				}, ""),

				Entry("which contains an empty rootfs path (even when asking for the default user)", func() {
					fakeBundleGetter.GetBundleReturns(&goci.Bndl{}, nil)
				}, ""),
			)

		})

		Context("When /etc/passwd cannot be parsed", func() {
			It("propagates an error", func() {
				graphPath := "graph-"
				fakeBundleGetter.GetBundleReturns(&goci.Bndl{Spec: specs.LinuxSpec{Spec: specs.Spec{Root: specs.Root{Path: graphPath}}}}, nil)
				fakePasswdFileParser.ParsePasswdFileReturns([]user.User{}, fmt.Errorf("unknown"))
				_, _, err := idgetter.GetIDs(ContainerID, "devil")
				Expect(err).To(MatchError("unknown"))
			})
		})

	})

})
