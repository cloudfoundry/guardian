package goci_test

import (
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Commands", func() {
	var binary goci.RuncBinary

	BeforeEach(func() {
		binary = goci.RuncBinary{Path: "funC", Root: "fancy-root"}
	})

	Describe("RunCommand", func() {
		It("creates an *exec.Cmd to create a container", func() {
			cmd := binary.RunCommand(
				"my-bundle-path",
				"my-pidfile-path",
				"my-logfile-path",
				"my-container",
				[]string{"foo", "bar"},
			)
			Expect(cmd.Args).To(Equal([]string{
				"funC",
				"--debug",
				"--log", "my-logfile-path",
				"--log-format", "json",
				"foo",
				"bar",
				"run",
				"--detach",
				"--no-new-keyring",
				"--bundle", "my-bundle-path",
				"--pid-file", "my-pidfile-path",
				"my-container",
			}))
		})
	})

	Describe("StartCommand", func() {
		It("creates an *exec.Cmd to start a bundle", func() {
			cmd := binary.StartCommand("my-bundle-path", "my-bundle-id", false, "mylog.file")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "--debug", "--log", "mylog.file", "--log-format", "json", "start", "my-bundle-id"}))
			Expect(cmd.Dir).To(Equal("my-bundle-path"))
		})

		It("turns on debug logging", func() {
			cmd := binary.StartCommand("my-bundle-path", "my-bundle-id", true, "mylog.file")
			Expect(cmd.Args).To(ContainElement("--debug"))
		})

		It("passes the detach flag if requested", func() {
			cmd := binary.StartCommand("my-bundle-path", "my-bundle-id", true, "mylog.file")
			Expect(cmd.Args).To(ContainElement("-d"))
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.StartCommand("my-bundle-path", "my-bundle-id", false, "mylog.file")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})

	Describe("ExecCommand", func() {
		It("creates an *exec.Cmd to exec a process in a bundle", func() {
			cmd := binary.ExecCommand("my-bundle-id", "my-process-json.json", "some-pid-file")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "exec", "my-bundle-id", "--pid-file", "some-pid-file", "-p", "my-process-json.json"}))
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.ExecCommand("my-bundle-id", "my-process-json.json", "some-pid-file")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})

	Describe("EventsCommand", func() {
		It("creates an *exec.Cmd to get events for a bundle", func() {
			cmd := binary.EventsCommand("my-bundle-id")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "events", "my-bundle-id"}))
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.EventsCommand("my-bundle-id")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})

	Describe("KillCommand", func() {
		It("creates an *exec.Cmd to signal the bundle", func() {
			cmd := binary.KillCommand("my-bundle-id", "TERM", "log.file")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "--debug", "--log", "log.file", "--log-format", "json", "kill", "my-bundle-id", "TERM"}))
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.KillCommand("my-bundle-id", "TERM", "log.file")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})

	Describe("StateCommand", func() {
		It("creates an *exec.Cmd to get the state of the bundle", func() {
			cmd := binary.StateCommand("my-bundle-id", "log.file")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "--debug", "--log", "log.file", "--log-format", "json", "state", "my-bundle-id"}))
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.StateCommand("my-bundle-id", "log.file")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})

	Describe("StatsCommand", func() {
		It("creates an *exec.Cmd to get the state of the bundle", func() {
			cmd := binary.StatsCommand("my-bundle-id", "log.file")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "--debug", "--log", "log.file", "--log-format", "json", "events", "--stats", "my-bundle-id"}))
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.StatsCommand("my-bundle-id", "log.file")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})

	Describe("DeleteCommand", func() {
		It("creates an *exec.Cmd to delete the bundle", func() {
			cmd := binary.DeleteCommand("my-bundle-id", false, "log.file")
			Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "--debug", "--log", "log.file", "--log-format", "json", "delete", "my-bundle-id"}))
		})

		Context("when forced", func() {
			It("passes the force flag to runc", func() {
				cmd := binary.DeleteCommand("my-bundle-id", true, "log.file")
				Expect(cmd.Args).To(Equal([]string{"funC", "--root", "fancy-root", "--debug", "--log", "log.file", "--log-format", "json", "delete", "--force", "my-bundle-id"}))
			})
		})

		Context("when runcroot is not set", func() {
			BeforeEach(func() {
				binary = goci.RuncBinary{Path: "funC"}
			})

			It("does not pass the root flag", func() {
				cmd := binary.DeleteCommand("my-bundle-id", false, "log.file")
				Expect(cmd.Args).NotTo(ContainElement("--root"))
			})
		})
	})
})
