// +build linux

package configure

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/docker/docker/pkg/reexec"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/devices"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"code.cloudfoundry.org/lager"
)

func init() {
	reexec.Register("configure-container-netns", func() {
		var netNsPath, containerIntf, containerIPStr, bridgeIPStr, subnetStr string
		var mtu int

		flag.StringVar(&netNsPath, "netNsPath", "", "netNsPath")
		flag.StringVar(&containerIntf, "containerIntf", "", "containerIntf")
		flag.StringVar(&containerIPStr, "containerIP", "", "containerIP")
		flag.StringVar(&bridgeIPStr, "bridgeIP", "", "bridgeIP")
		flag.StringVar(&subnetStr, "subnet", "", "subnet")
		flag.IntVar(&mtu, "mtu", 0, "mtu")
		flag.Parse()

		fd, err := os.Open(netNsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opening netns `%s`: %s", netNsPath, err)
			os.Exit(1)
		}
		defer fd.Close()

		netNsExecer := &netns.Execer{}

		if err = netNsExecer.Exec(fd, func() error {
			containerIP := net.ParseIP(containerIPStr)
			bridgeIP := net.ParseIP(bridgeIPStr)
			_, subnetIPNet, err := net.ParseCIDR(subnetStr)
			if err != nil {
				panic(err)
			}

			link := devices.Link{}

			intf, found, err := link.InterfaceByName(containerIntf)
			if err != nil {
				panic(err)
			}
			if !found {
				return fmt.Errorf("interface `%s` was not found", containerIntf)
			}

			if err := link.AddIP(intf, containerIP, subnetIPNet); err != nil {
				panic(err)
			}

			if err := link.SetUp(intf); err != nil {
				panic(err)
			}

			if err := link.AddDefaultGW(intf, bridgeIP); err != nil {
				panic(err)
			}

			if err := link.SetMTU(intf, mtu); err != nil {
				panic(err)
			}

			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	})
}

type Container struct {
	FileOpener netns.Opener
}

func (c *Container) Apply(log lager.Logger, cfg kawasaki.NetworkConfig, pid int) error {
	netns, err := c.FileOpener.Open(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return err
	}
	defer netns.Close()

	log = log.Session("configure-container-netns", lager.Data{
		"networkConfig": cfg,
		"netNsPath":     netns.Name(),
	})

	cmd := reexec.Command("configure-container-netns",
		"-netNsPath", netns.Name(),
		"-containerIntf", cfg.ContainerIntf,
		"-containerIP", cfg.ContainerIP.String(),
		"-bridgeIP", cfg.BridgeIP.String(),
		"-subnet", cfg.Subnet.String(),
		"-mtu", strconv.FormatInt(int64(cfg.Mtu), 10),
	)

	errBuf := bytes.NewBuffer([]byte{})
	cmd.Stderr = errBuf

	if err := cmd.Start(); err != nil {
		log.Error("starting-command", errors.New(errBuf.String()))
		return err
	}

	if err := cmd.Wait(); err != nil {
		status, err := exitStatus(err)
		if err != nil {
			log.Error("waiting-for-command", errors.New(errBuf.String()))
			return err
		}

		if status == 1 {
			return errors.New(errBuf.String())
		}

		log.Error("unexpected-error", errors.New(errBuf.String()))
		return errors.New("unexpected error")
	}

	return nil
}

func exitStatus(err error) (uint32, error) {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 2, err
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return 2, exitErr
	}

	return uint32(status.ExitStatus()), nil
}
