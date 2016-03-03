package bundlerules_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/specs"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
)

var _ = Describe("Hooks", func() {
	DescribeTable("the envirionment should contain", func(envVar string) {
		rule := bundlerules.Hooks{LogFilePattern: "/path/to/%s.log"}

		newBndl := rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Handle: "fred",
		})

		Expect(newBndl.PrestartHooks()[0].Env).To(
			ContainElement(envVar),
		)
	},
		Entry("the GARDEN_LOG_FILE path", "GARDEN_LOG_FILE=/path/to/fred.log"),
		Entry("a sensible PATH", "PATH="+os.Getenv("PATH")),
	)

	It("adds the prestart and poststop hooks of the passed bundle", func() {
		newBndl := bundlerules.Hooks{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			NetworkHooks: gardener.Hooks{
				Prestart: gardener.Hook{
					Path: "/path/to/bananas/network",
					Args: []string{"arg", "barg"},
				},
				Poststop: gardener.Hook{
					Path: "/path/to/bananas/network",
					Args: []string{"arg", "barg"},
				},
			},
		})

		Expect(pathAndArgsOf(newBndl.PrestartHooks())).To(ContainElement(PathAndArgs{
			Path: "/path/to/bananas/network",
			Args: []string{"arg", "barg"},
		}))

		Expect(pathAndArgsOf(newBndl.PoststopHooks())).To(ContainElement(PathAndArgs{
			Path: "/path/to/bananas/network",
			Args: []string{"arg", "barg"},
		}))
	})

	It("does not include a post-stop hook if none was requested", func() {
		newBndl := bundlerules.Hooks{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			NetworkHooks: gardener.Hooks{
				Prestart: gardener.Hook{
					Path: "/path/to/bananas/network",
					Args: []string{"arg", "barg"},
				},
			},
		})

		Expect(pathAndArgsOf(newBndl.PoststopHooks())).To(BeEmpty())
	})
})

func pathAndArgsOf(a []specs.Hook) (b []PathAndArgs) {
	for _, h := range a {
		b = append(b, PathAndArgs{h.Path, h.Args})
	}

	return
}

type PathAndArgs struct {
	Path string
	Args []string
}
