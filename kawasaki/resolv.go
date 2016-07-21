package kawasaki

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ResolvFileCompiler
type ResolvFileCompiler interface {
	Compile(log lager.Logger, resolvConfPath string, containerIp net.IP, overrideServers []net.IP) ([]byte, error)
}

//go:generate counterfeiter . HostFileCompiler
type HostFileCompiler interface {
	Compile(log lager.Logger, containerIp net.IP, handle string) ([]byte, error)
}

//go:generate counterfeiter . FileWriter
type FileWriter interface {
	WriteFile(log lager.Logger, filePath string, contents []byte, rootfsPath string, rootUid, rootGid int) error
}

//go:generate counterfeiter . IdMapReader
type IdMapReader interface {
	ReadRootId(path string) (int, error)
}

type ResolvConfigurer struct {
	HostsFileCompiler  HostFileCompiler
	ResolvFileCompiler ResolvFileCompiler
	FileWriter         FileWriter
	IDMapReader        IdMapReader
}

type RootIdMapReader struct{}

// Reads /proc/<pid>/{uid_map|gid_map} and retrieves the mapped user id
// for the container root user. Map format is the following:
// containerId hostId mappingSize
// 0           1000   1
func (r *RootIdMapReader) ReadRootId(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return -1, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')
		fields := strings.Fields(line)

		if len(fields) > 1 && fields[0] == "0" {
			return strconv.Atoi(fields[1])
		}

		if err != nil {
			break
		}
	}
	return -1, errors.New("no root mapping")
}

func (d *ResolvConfigurer) Configure(log lager.Logger, cfg NetworkConfig, pid int) error {
	log = log.Session("dns-resolve-configure")

	contents, err := d.HostsFileCompiler.Compile(log, cfg.ContainerIP, cfg.ContainerHandle)
	if err != nil {
		log.Error("compiling-hosts-file", err)
		return err
	}

	rootUid, err := d.IDMapReader.ReadRootId(fmt.Sprintf("/proc/%d/uid_map", pid))
	if err != nil {
		return err
	}

	rootGid, err := d.IDMapReader.ReadRootId(fmt.Sprintf("/proc/%d/gid_map", pid))
	if err != nil {
		return err
	}

	if err := d.FileWriter.WriteFile(log, "/etc/hosts", contents, fmt.Sprintf("/proc/%d/root", pid), rootUid, rootGid); err != nil {
		log.Error("writting-hosts-file", err)
		return fmt.Errorf("writting file '/etc/hosts': %s", err)
	}

	contents, err = d.ResolvFileCompiler.Compile(log, "/etc/resolv.conf", cfg.BridgeIP, cfg.DNSServers)
	if err != nil {
		log.Error("compiling-resolv-file", err)
		return err
	}

	if err := d.FileWriter.WriteFile(log, "/etc/resolv.conf", contents, fmt.Sprintf("/proc/%d/root", pid), rootUid, rootGid); err != nil {
		log.Error("writting-resolv-file", err)
		return fmt.Errorf("writting file '/etc/resolv.conf': %s", err)
	}

	return nil
}
