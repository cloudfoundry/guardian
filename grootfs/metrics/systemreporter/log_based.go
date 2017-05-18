package systemreporter

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/lager"
)

type LogBased struct {
	threshold time.Duration
	cmdRunner commandrunner.CommandRunner
}

func NewLogBased(threshold time.Duration, cmdRunner commandrunner.CommandRunner) *LogBased {
	return &LogBased{
		threshold: threshold,
		cmdRunner: cmdRunner,
	}
}

func (r *LogBased) Report(logger lager.Logger, duration time.Duration) {
	if r.threshold <= 0 || duration < r.threshold {
		return
	}

	logger = logger.Session("system-reporter", lager.Data{"duration": duration})
	report := Report{
		PidStat:              r.execute("pidstat"),
		VmStat:               r.execute("vmstat"),
		MpStat:               r.execute("mpstat", "-P", "ALL"),
		IoStat:               r.execute("iostat", "-xzp"),
		TopProcessesByMemory: r.topEntries(10, r.execute("ps", "-aux", "--sort", "-rss")),
		TopProcessesByCPU:    r.topEntries(10, r.execute("ps", "-aux", "--sort", "-pcpu")),
		Dmesg:                r.tailEntries(250, r.execute("dmesg")),
	}

	logger.Info("threshold-reached", lager.Data{"report": report})
}

func (r *LogBased) execute(args ...string) string {
	buffer := bytes.NewBuffer([]byte{})
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = buffer
	cmd.Stderr = buffer

	if err := r.cmdRunner.Run(cmd); err != nil {
		return fmt.Sprintf("Failed to fetch %v: %s", args, buffer.String())
	}

	return buffer.String()
}

func (r *LogBased) tailEntries(n int, entries string) string {
	allEntries := strings.Split(entries, "\n")

	if n > len(allEntries) {
		return entries
	}
	n = len(allEntries) - n
	bottomEntries := allEntries[n:]
	return strings.Join(bottomEntries, "\n")
}
func (r *LogBased) topEntries(n int, entries string) string {
	allEntries := strings.Split(entries, "\n")

	if n > len(allEntries) {
		return entries
	}

	topEntries := allEntries[:n]
	return strings.Join(topEntries, "\n")
}
