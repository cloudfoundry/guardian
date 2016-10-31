package iptables

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/guardian/pkg/locksmith"

	"github.com/cloudfoundry/gunk/command_runner"
)

const LockKey = "/var/run/garden-iptables.lock"

type Locksmith interface {
	Lock(key string) (locksmith.Unlocker, error)
}

//go:generate counterfeiter . Rule
type Rule interface {
	Flags(chain string) []string
}

//go:generate counterfeiter . IPTables
type IPTables interface {
	CreateChain(table, chain string) error
	DeleteChain(table, chain string) error
	FlushChain(table, chain string) error
	DeleteChainReferences(table, targetChain, referencedChain string) error
	PrependRule(chain string, rule Rule) error
	BulkPrependRules(chain string, rules []Rule) error
	InstanceChain(instanceId string) string
}

type IPTablesController struct {
	runner                                                                                         command_runner.CommandRunner
	locksmith                                                                                      Locksmith
	binPath                                                                                        string
	preroutingChain, postroutingChain, inputChain, forwardChain, defaultChain, instanceChainPrefix string
}

type Chains struct {
	Prerouting, Postrouting, Input, Forward, Default string
}

func New(binPath string, runner command_runner.CommandRunner, locksmith Locksmith, chainPrefix string) *IPTablesController {
	return &IPTablesController{
		runner:    runner,
		locksmith: locksmith,
		binPath:   binPath,

		preroutingChain:     chainPrefix + "prerouting",
		postroutingChain:    chainPrefix + "postrouting",
		inputChain:          chainPrefix + "input",
		forwardChain:        chainPrefix + "forward",
		defaultChain:        chainPrefix + "default",
		instanceChainPrefix: chainPrefix + "instance-",
	}
}

func (iptables *IPTablesController) CreateChain(table, chain string) error {
	return iptables.run("create-instance-chains", exec.Command(iptables.binPath, "--wait", "--table", table, "-N", chain))
}

func (iptables *IPTablesController) DeleteChain(table, chain string) error {
	shellCmd := fmt.Sprintf(
		`%s --wait --table %s -X %s 2> /dev/null || true`,
		iptables.binPath, table, chain,
	)
	return iptables.run("delete-instance-chains", exec.Command("sh", "-c", shellCmd))
}

func (iptables *IPTablesController) FlushChain(table, chain string) error {
	shellCmd := fmt.Sprintf(
		`%s --wait --table %s -F %s 2> /dev/null || true`,
		iptables.binPath, table, chain,
	)
	return iptables.run("flush-instance-chains", exec.Command("sh", "-c", shellCmd))
}

func (iptables *IPTablesController) DeleteChainReferences(table, targetChain, referencedChain string) error {
	shellCmd := fmt.Sprintf(
		`set -e; %s --wait --table %s -S %s | grep "%s" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 %s -w -t %s`,
		iptables.binPath, table, targetChain, referencedChain, iptables.binPath, table,
	)
	return iptables.run("delete-referenced-chains", exec.Command("sh", "-c", shellCmd))
}

func (iptables *IPTablesController) PrependRule(chain string, rule Rule) error {
	return iptables.run("prepend", exec.Command(iptables.binPath, append([]string{"-w", "-I", chain, "1"}, rule.Flags(chain)...)...))
}

func (iptables *IPTablesController) BulkPrependRules(chain string, rules []Rule) error {
	in := bytes.NewBuffer([]byte{})
	in.WriteString("*filter\n")
	for _, r := range rules {
		in.WriteString(fmt.Sprintf("-I %s 1 ", chain))
		in.WriteString(strings.Join(r.Flags(chain), " "))
		in.WriteString("\n")
	}
	in.WriteString("COMMIT\n")

	cmd := exec.Command("iptables-restore", "--noflush")
	cmd.Stdin = in

	return iptables.run("append-rules", cmd)
}

func (iptables *IPTablesController) InstanceChain(instanceId string) string {
	return iptables.instanceChainPrefix + instanceId
}

func (iptables *IPTablesController) run(action string, cmd *exec.Cmd) error {
	var buff bytes.Buffer
	cmd.Stdout = &buff
	cmd.Stderr = &buff

	u, err := iptables.locksmith.Lock(LockKey)
	if err != nil {
		return err
	}

	if err := iptables.runner.Run(cmd); err != nil {
		err := fmt.Errorf("%s %s: %s", iptables.binPath, action, buff.String())
		if unlockErr := u.Unlock(); unlockErr != nil {
			err = fmt.Errorf("%s and then %s", err, unlockErr)
		}
		return err
	}
	return u.Unlock()
}

func (iptables *IPTablesController) appendRule(chain string, rule Rule) error {
	return iptables.run("append", exec.Command(iptables.binPath, append([]string{"-w", "-A", chain}, rule.Flags(chain)...)...))
}
