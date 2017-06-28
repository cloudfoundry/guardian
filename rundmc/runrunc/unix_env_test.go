package runrunc_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("UnixEnvFor", func() {
	Context("when the user is specified in the process spec", func() {
		DescribeTable("appends the correct USER env var", func(specEnv, expectedEnv []string) {
			env := runrunc.UnixEnvFor(2, goci.Bndl{}, garden.ProcessSpec{
				User: "spiderman",
				Env:  specEnv,
			})

			Expect(env).To(Equal(expectedEnv))
		},
			Entry(
				"when Env does not contain USER",
				[]string{"a=1", "PATH=a", "HOME=/spidermanhome"},
				[]string{"a=1", "PATH=a", "HOME=/spidermanhome", "USER=spiderman"},
			),
			Entry(
				"when Env does not contain USER, but does contain an env var matching the string .*USER",
				[]string{"a=1", "PATH=a", "AUSER=foo"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USER=spiderman"},
			),
			Entry(
				"when Env does not contain USER, but does contain an env var matching the string USER.*",
				[]string{"a=1", "PATH=a", "USERA=bar"},
				[]string{"a=1", "PATH=a", "USERA=bar", "USER=spiderman"},
			),
			Entry(
				"when Env does not contain USER, but does contain an env var matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSERB=baz"},
				[]string{"a=1", "PATH=a", "AUSERB=baz", "USER=spiderman"},
			),
			Entry(
				"when Env does not contain USER, but contains many env vars matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg", "USER=spiderman"},
			),
			Entry(
				"when Env does contain USER",
				[]string{"a=1", "PATH=a", "USER=superman"},
				[]string{"a=1", "PATH=a", "USER=superman"},
			),
			Entry(
				"when Env does contain USER, as well as an env var matching the string .*USER",
				[]string{"a=1", "PATH=a", "AUSER=foo", "USER=superman"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USER=superman"},
			),
			Entry(
				"when Env does contain USER, as well as an env var matching the string USER.*",
				[]string{"a=1", "PATH=a", "USERA=bar", "USER=superman"},
				[]string{"a=1", "PATH=a", "USERA=bar", "USER=superman"},
			),
			Entry(
				"when Env does contain USER, as well as an env var matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSERB=baz", "USER=superman"},
				[]string{"a=1", "PATH=a", "AUSERB=baz", "USER=superman"},
			),
			Entry(
				"when Env does contain USER, as well as many env vars matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg", "USER=superman"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg", "USER=superman"},
			),
		)
	})

	Context("when the user is not specified in the process spec", func() {
		DescribeTable("appends the correct USER env var", func(specEnv, expectedEnv []string) {
			env := runrunc.UnixEnvFor(1, goci.Bndl{}, garden.ProcessSpec{
				Env: specEnv,
			})

			Expect(env).To(Equal(expectedEnv))
		},
			Entry(
				"when Env does not contain USER",
				[]string{"a=1", "PATH=a"},
				[]string{"a=1", "PATH=a", "USER=root"},
			),
			Entry(
				"when Env does not contain USER, but does contain an env var matching the string .*USER",
				[]string{"a=1", "PATH=a", "AUSER=foo"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USER=root"},
			),
			Entry(
				"when Env does not contain USER, but does contain an env var matching the string USER.*",
				[]string{"a=1", "PATH=a", "USERA=bar"},
				[]string{"a=1", "PATH=a", "USERA=bar", "USER=root"},
			),
			Entry(
				"when Env does not contain USER, but does contain an env var matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSERB=baz"},
				[]string{"a=1", "PATH=a", "AUSERB=baz", "USER=root"},
			),
			Entry(
				"when Env does not contain USER, but contains many env vars matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg", "USER=root"},
			),
			Entry(
				"when Env does contain USER",
				[]string{"a=1", "PATH=a", "USER=yo"},
				[]string{"a=1", "PATH=a", "USER=yo"},
			),
			Entry(
				"when Env does contain USER, as well as an env var matching the string .*USER",
				[]string{"a=1", "PATH=a", "AUSER=foo", "USER=yo"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USER=yo"},
			),
			Entry(
				"when Env does contain USER, as well as an env var matching the string USER.*",
				[]string{"a=1", "PATH=a", "USERA=bar", "USER=yo"},
				[]string{"a=1", "PATH=a", "USERA=bar", "USER=yo"},
			),
			Entry(
				"when Env does contain USER, as well as an env var matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSERB=baz", "USER=yo"},
				[]string{"a=1", "PATH=a", "AUSERB=baz", "USER=yo"},
			),
			Entry(
				"when Env does contain USER, as well as many env vars matching the string .*USER.*",
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg", "USER=yo"},
				[]string{"a=1", "PATH=a", "AUSER=foo", "USERB=bar", "USERINYOURUSER=yodawg", "USER=yo"},
			),
		)
	})

	Context("when the environment already contains a PATH", func() {
		It("passes the environment variables", func() {
			env := runrunc.UnixEnvFor(1, goci.Bndl{}, garden.ProcessSpec{
				Env: []string{"a=1", "b=3", "c=4", "PATH=a"},
			})

			Expect(env).To(Equal([]string{"a=1", "b=3", "c=4", "PATH=a", "USER=root"}))
		})
	})

	Context("when the environment does not already contain a PATH", func() {
		DescribeTable("appends a default PATH", func(procUser string, uid int, specEnv, expectedEnv []string) {
			env := runrunc.UnixEnvFor(uid, goci.Bndl{}, garden.ProcessSpec{
				Env:  specEnv,
				User: procUser,
			})

			Expect(env).To(Equal(expectedEnv))
		},
			Entry(
				"for the root user", "root", 0,
				[]string{"a=1"},
				[]string{"a=1", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "USER=root"},
			),
			Entry(
				"for the root user, and an env var matching the string .*PATH", "root", 0,
				[]string{"a=1", "APATH=foo"},
				[]string{"a=1", "APATH=foo", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "USER=root"},
			),
			Entry(
				"for the root user, and an env var matching the string PATH.*", "root", 0,
				[]string{"a=1", "PATHA=bar"},
				[]string{"a=1", "PATHA=bar", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "USER=root"},
			),
			Entry(
				"for the root user, and an env var matching the string .*PATH.*", "root", 0,
				[]string{"a=1", "APATHB=baz"},
				[]string{"a=1", "APATHB=baz", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "USER=root"},
			),
			Entry(
				"for the root user, and many env vars matching the string .*PATH.*", "root", 0,
				[]string{"a=1", "APATH=foo", "PATHB=bar", "PATHINYOURPATH=yodawg"},
				[]string{"a=1", "APATH=foo", "PATHB=bar", "PATHINYOURPATH=yodawg", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "USER=root"},
			),
			Entry(
				"for a non-root user", "alice", 1000,
				[]string{"a=1"},
				[]string{"a=1", "PATH=/usr/local/bin:/usr/bin:/bin", "USER=alice"},
			),
			Entry(
				"for a non-root user, and an env var matching the string .*PATH", "alice", 1000,
				[]string{"a=1", "APATH=foo"},
				[]string{"a=1", "APATH=foo", "PATH=/usr/local/bin:/usr/bin:/bin", "USER=alice"},
			),
			Entry(
				"for a non-root user, and an env var matching the string PATH.*", "alice", 1000,
				[]string{"a=1", "PATHA=bar"},
				[]string{"a=1", "PATHA=bar", "PATH=/usr/local/bin:/usr/bin:/bin", "USER=alice"},
			),
			Entry(
				"for a non-root user, and an env var matching the string .*PATH.*", "alice", 1000,
				[]string{"a=1", "APATHB=baz"},
				[]string{"a=1", "APATHB=baz", "PATH=/usr/local/bin:/usr/bin:/bin", "USER=alice"},
			),
			Entry(
				"for a non-root user, and many env vars matching the string .*PATH.*", "alice", 1000,
				[]string{"a=1", "APATH=foo", "PATHB=bar", "PATHINYOURPATH=yodawg"},
				[]string{"a=1", "APATH=foo", "PATHB=bar", "PATHINYOURPATH=yodawg", "PATH=/usr/local/bin:/usr/bin:/bin", "USER=alice"},
			),
		)
	})

	Context("when the container bundle has environment variables", func() {
		var (
			processEnv   []string
			containerEnv []string
			bndl         goci.Bndl

			env []string
		)

		BeforeEach(func() {
			containerEnv = []string{"ENV_CONTAINER_NAME=garden"}
			processEnv = []string{"ENV_PROCESS_ID=1"}
		})

		JustBeforeEach(func() {
			bndl = goci.Bundle()
			bndl.Spec.Process.Env = containerEnv

			env = runrunc.UnixEnvFor(9, bndl, garden.ProcessSpec{
				Env: processEnv,
			})
		})

		It("appends the process vars into container vars", func() {
			envWContainer := make([]string, len(env))
			copy(envWContainer, env)

			bndl.Spec.Process.Env = []string{}

			env = runrunc.UnixEnvFor(9, bndl, garden.ProcessSpec{
				Env: processEnv,
			})

			Expect(envWContainer).To(Equal(append(containerEnv, env...)))
		})

		Context("and the container environment contains PATH", func() {
			BeforeEach(func() {
				containerEnv = append(containerEnv, "PATH=/test")
			})

			It("should not apply the default PATH", func() {
				Expect(env).To(Equal([]string{
					"ENV_CONTAINER_NAME=garden",
					"PATH=/test",
					"ENV_PROCESS_ID=1",
					"USER=root",
				}))
			})
		})
	})
})
