package dns

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Compiler
type Compiler interface {
	Compile(log lager.Logger) ([]byte, error)
}

//go:generate counterfeiter . FileWriter
type FileWriter interface {
	WriteFile(log lager.Logger, filePath string, contents []byte) error
}

type ResolvConfigurer struct {
	HostsFileCompiler  Compiler
	ResolvFileCompiler Compiler
	FileWriter         FileWriter
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

func (d *ResolvConfigurer) Configure(log lager.Logger) error {
	log = log.Session("dns-resolve-configure")

	contents, err := d.HostsFileCompiler.Compile(log)
	if err != nil {
		log.Error("compiling-hosts-file", err)
		return err
	}

	if err := d.FileWriter.WriteFile(log, "/etc/hosts", contents); err != nil {
		log.Error("writting-hosts-file", err)
		return fmt.Errorf("writting file '/etc/hosts': %s", err)
	}

	contents, err = d.ResolvFileCompiler.Compile(log)
	if err != nil {
		log.Error("compiling-resolv-file", err)
		return err
	}

	if err := d.FileWriter.WriteFile(log, "/etc/resolv.conf", contents); err != nil {
		log.Error("writting-resolv-file", err)
		return fmt.Errorf("writting file '/etc/resolv.conf': %s", err)
	}

	return nil
}
