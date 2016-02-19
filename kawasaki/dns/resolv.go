package dns

import (
	"fmt"

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
