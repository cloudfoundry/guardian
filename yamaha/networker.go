package yamaha

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"code.cloudfoundry.org/lager"
)

type Networker struct {
	NetnsExecer *netns.Execer
}

func (n *Networker) Network(log lager.Logger, spec garden.ContainerSpec, pid int) error {

	// 1. Garden sets up cable to outside: vde_plug -d vxvde:// slirp://
	// 2. Create a container
	//    - In the container create a tap device and set it to UP
	// 3. Garden connects vxvde:// to tap: vde_plug vxvde:// = nsenter -- -t $(runc state foo | jq -r .pid) -n -U --preserve-credentials vde_plug tap://tap0
	// 4. Set up route in container:
	//    - ip link set tap0 up
	//    - ip addr add 10.0.2.100/24 dev tap0
	//    - ip route add default via 10.0.2.2 dev tap0

	hostCableCmd := exec.Command("vde_plug", "-d", "vxvde://", "slirp://tcpfwd=112233:10.0.2.100:80")
	if output, err := hostCableCmd.CombinedOutput(); err != nil {
		log.Error("host-cable-cmd", err, lager.Data{"output": string(output)})
		return err
	}

	f, err := os.Open(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return err
	}
	defer f.Close()

	output, err := runInNS(log, []string{"cat", "/proc/self/status"}, pid)
	if err != nil {
		log.Error("caps", err, lager.Data{"output": string(output)})
		return err
	}

	log.Info("caps", lager.Data{"output": string(output)})

	if output, err := runInNS(log, []string{"ip", "tuntap", "add", "name", "tap0", "mode", "tap"}, pid); err != nil {
		log.Error("create-tap-cmd", err, lager.Data{"output": string(output)})
		return err
	}

	if output, err := setTapLinkUp(log, pid); err != nil {
		log.Error("set-tap-link-up", err, lager.Data{"output": string(output)})
		return err
	}

	xnsCableCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("vde_plug -d vxvde:// = nsenter -- -t %d -n -U --preserve-credentials vde_plug tap://tap0", pid))
	log.Info("xns-cable-args", lager.Data{"cmd": strings.Join(xnsCableCmd.Args, " ")})
	if output, err := xnsCableCmd.CombinedOutput(); err != nil {
		log.Error("xns-cable-cmd", err, lager.Data{"output": string(output)})
		return err
	}

	if output, err := setTapLinkUp(log, pid); err != nil {
		log.Error("set-tap-link-up-2", err, lager.Data{"output": string(output)})
		return err
	}

	if output, err := runInNS(log, []string{"ip", "addr", "add", "10.0.2.100/24", "dev", "tap0"}, pid); err != nil {
		log.Error("add-ip-cmd", err, lager.Data{"output": string(output)})
		return err
	}

	if output, err := runInNS(log, []string{"ip", "route", "add", "default", "via", "10.0.2.2", "dev", "tap0"}, pid); err != nil {
		log.Error("add-default-root-cmd", err, lager.Data{"output": string(output)})
		return err
	}

	if err := writeExistingFile(filepath.Join("/var/vcap/data/garden/depot", spec.Handle, "resolv.conf"), []byte("nameserver 10.0.2.3")); err != nil {
		log.Error("writing-resolv-file", err)
		return err
	}

	return nil
}

func runInNS(log lager.Logger, args []string, pid int) ([]byte, error) {
	args = append([]string{"-t", strconv.Itoa(pid), "-n", "-U", "--preserve-credentials"}, args...)
	log.Info("run-in-ns", lager.Data{"cmd": strings.Join(args, " ")})
	cmd := exec.Command("nsenter", args...)
	return cmd.CombinedOutput()
}

func setTapLinkUp(log lager.Logger, pid int) ([]byte, error) {
	return runInNS(log, []string{"ip", "link", "set", "tap0", "up"}, pid)
}

func writeExistingFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return err
	}
	return nil
}

func (n *Networker) Capacity() uint64 {
	return 0
}

func (n *Networker) Destroy(log lager.Logger, handle string) error {
	return nil
}

func (n *Networker) NetIn(log lager.Logger, handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (n *Networker) BulkNetOut(log lager.Logger, handle string, rules []garden.NetOutRule) error {
	return nil
}

func (n *Networker) NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error {
	return nil
}

func (n *Networker) Restore(log lager.Logger, handle string) error {
	return nil
}
