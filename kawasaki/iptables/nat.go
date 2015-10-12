package iptables

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type natChain struct {
	cfg    NATConfig
	runner command_runner.CommandRunner
	logger lager.Logger
}

func NewNATChain(cfg NATConfig, runner command_runner.CommandRunner, logger lager.Logger) *natChain {
	return &natChain{
		cfg:    cfg,
		runner: runner,
		logger: logger,
	}
}

func (mgr *natChain) Setup(instanceChain, bridgeName string, ip net.IP, network *net.IPNet) error {
	commands := []*exec.Cmd{
		// Create nat instance chain
		exec.Command("iptables", "--wait", "--table", "nat", "-N", instanceChain),
		// Bind nat instance chain to nat prerouting chain
		exec.Command("iptables", "--wait", "--table", "nat", "-A", mgr.cfg.PreroutingChain, "--jump", instanceChain),
		// Enable NAT for traffic coming from containers
		exec.Command("sh", "-c", fmt.Sprintf(
			`(iptables --wait --table nat -S %s | grep "\-j MASQUERADE\b" | grep -q -F -- "-s %s") || iptables --wait --table nat -A %s --source %s ! --destination %s --jump MASQUERADE`,
			mgr.cfg.PostroutingChain, network.String(), mgr.cfg.PostroutingChain,
			network.String(), network.String(),
		)),
	}

	for _, cmd := range commands {
		if err := mgr.runner.Run(cmd); err != nil {
			buffer := &bytes.Buffer{}
			cmd.Stderr = buffer
			logger := mgr.logger.Session("setup", lager.Data{"cmd": cmd})
			logger.Debug("starting")
			if err := mgr.runner.Run(cmd); err != nil {
				stderr, _ := ioutil.ReadAll(buffer)
				logger.Error("failed", err, lager.Data{"stderr": string(stderr)})
				return fmt.Errorf("iptables_manager: nat: %s", err)
			}
			logger.Debug("ended")
		}
	}

	return nil
}

func (mgr *natChain) Teardown(instanceChain string) error {
	commands := []*exec.Cmd{
		// Prune nat prerouting chain
		exec.Command("sh", "-c", fmt.Sprintf(
			`iptables --wait --table nat -S %s 2> /dev/null | grep "\-j %s\b" | sed -e "s/-A/-D/" | xargs --no-run-if-empty --max-lines=1 iptables --wait --table nat`,
			mgr.cfg.PreroutingChain, instanceChain,
		)),
		// Flush nat instance chain
		exec.Command("sh", "-c", fmt.Sprintf(`iptables --wait --table nat -F %s 2> /dev/null || true`, instanceChain)),
		// Delete nat instance chain
		exec.Command("sh", "-c", fmt.Sprintf(`iptables --wait --table nat -X %s 2> /dev/null || true`, instanceChain)),
	}

	for _, cmd := range commands {
		buffer := &bytes.Buffer{}
		cmd.Stderr = buffer
		logger := mgr.logger.Session("teardown", lager.Data{"cmd": cmd})
		logger.Debug("starting")
		if err := mgr.runner.Run(cmd); err != nil {
			stderr, _ := ioutil.ReadAll(buffer)
			logger.Error("failed", err, lager.Data{"stderr": string(stderr)})
			return fmt.Errorf("iptables_manager: nat: %s", err)
		}
		logger.Debug("ended")
	}

	return nil
}
