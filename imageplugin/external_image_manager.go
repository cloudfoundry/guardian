package imageplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/tscolari/lagregator"
)

type ExternalImageManager struct {
	binPath             string
	commandRunner       command_runner.CommandRunner
	defaultBaseImage    *url.URL
	mappings            []specs.LinuxIDMapping
	privilegedExtraArgs []string
	extraArgs           []string
}

func New(binPath string, commandRunner command_runner.CommandRunner,
	defaultBaseImage *url.URL, mappings []specs.LinuxIDMapping,
	privilegedExtraArgs []string,
	extraArgs []string,
) *ExternalImageManager {
	return &ExternalImageManager{
		binPath:             binPath,
		commandRunner:       commandRunner,
		defaultBaseImage:    defaultBaseImage,
		mappings:            mappings,
		privilegedExtraArgs: privilegedExtraArgs,
		extraArgs:           extraArgs,
	}
}

func (p *ExternalImageManager) Create(log lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error) {
	log = log.Session("image-plugin-create")
	log.Debug("start")
	defer log.Debug("end")

	var args []string
	if spec.Namespaced {
		args = append(p.extraArgs, "create")
	} else {
		args = append(p.privilegedExtraArgs, "create")
	}
	if spec.QuotaSize != 0 {
		if spec.QuotaScope == garden.DiskLimitScopeExclusive {
			args = append(args, "--exclude-image-from-quota")
		}
		args = append(args, "--disk-limit-size-bytes", strconv.FormatInt(spec.QuotaSize, 10))
	}

	if spec.Namespaced {
		for _, mapping := range p.mappings {
			args = append(args, "--uid-mapping", stringifyMapping(mapping))
			args = append(args, "--gid-mapping", stringifyMapping(mapping))
		}
	}

	if spec.RootFS == nil || spec.RootFS.String() == "" {
		args = append(args, p.defaultBaseImage.String())
	} else {
		args = append(args, strings.Replace(spec.RootFS.String(), "#", ":", 1))
	}

	args = append(args, handle)

	cmd := exec.Command(p.binPath, args...)

	cmd.Stderr = lagregator.NewRelogger(log)
	outBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuffer

	if spec.Namespaced {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: p.mappings[0].HostID,
				Gid: p.mappings[0].HostID,
			},
		}
	}

	if err := p.commandRunner.Run(cmd); err != nil {
		logData := lager.Data{"action": "create", "stdout": outBuffer.String()}
		log.Error("external-image-manager-result", err, logData)
		return "", nil, fmt.Errorf("external image manager create failed: %s (%s)", outBuffer.String(), err)
	}

	imagePath := strings.TrimSpace(outBuffer.String())
	envVars, err := p.readEnvVars(imagePath)
	if err != nil {
		return "", nil, err
	}

	rootFSPath := filepath.Join(imagePath, "rootfs")
	return rootFSPath, envVars, nil
}

func (p *ExternalImageManager) Destroy(log lager.Logger, handle, rootFSPath string) error {
	log = log.Session("image-plugin-destroy")
	log.Debug("start")
	defer log.Debug("end")

	imagePath := filepath.Dir(rootFSPath)
	args := append(p.extraArgs, "delete", imagePath)
	cmd := exec.Command(p.binPath, args...)

	cmd.Stderr = lagregator.NewRelogger(log)
	outBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuffer

	if err := p.commandRunner.Run(cmd); err != nil {
		logData := lager.Data{"action": "delete", "stdout": outBuffer.String()}
		log.Error("external-image-manager-result", err, logData)
		return fmt.Errorf("external image manager destroy failed: %s (%s)", outBuffer.String(), err)
	}

	return nil
}

func (p *ExternalImageManager) Metrics(log lager.Logger, _, rootfs string) (garden.ContainerDiskStat, error) {
	log = log.Session("image-plugin-metrics", lager.Data{"rootfs": rootfs})
	log.Debug("start")
	defer log.Debug("end")

	imagePath := filepath.Dir(rootfs)
	args := append(p.extraArgs, "stats", imagePath)
	cmd := exec.Command(p.binPath, args...)
	cmd.Stderr = lagregator.NewRelogger(log)
	outBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuffer

	if err := p.commandRunner.Run(cmd); err != nil {
		logData := lager.Data{"action": "stats", "stderr": outBuffer.String()}
		log.Error("external-image-manager-result", err, logData)
		return garden.ContainerDiskStat{}, fmt.Errorf("external image manager metrics failed: %s (%s)", outBuffer.String(), err)
	}

	var metrics map[string]map[string]uint64
	if err := json.NewDecoder(outBuffer).Decode(&metrics); err != nil {
		return garden.ContainerDiskStat{}, fmt.Errorf("parsing metrics: %s", err)
	}

	return garden.ContainerDiskStat{
		TotalBytesUsed:     metrics["disk_usage"]["total_bytes_used"],
		ExclusiveBytesUsed: metrics["disk_usage"]["exclusive_bytes_used"],
	}, nil
}

func (p *ExternalImageManager) GC(log lager.Logger) error {
	log = log.Session("image-plugin-gc")
	log.Debug("start")
	defer log.Debug("end")

	args := append(p.extraArgs, "clean")
	cmd := exec.Command(p.binPath, args...)
	cmd.Stderr = lagregator.NewRelogger(log)
	outBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outBuffer

	if err := p.commandRunner.Run(cmd); err != nil {
		logData := lager.Data{"action": "clean", "stdout": outBuffer.String()}
		log.Error("external-image-manager-result", err, logData)
		return fmt.Errorf("external image manager clean failed: %s (%s)", outBuffer.String(), err)
	}

	return nil
}

func stringifyMapping(mapping specs.LinuxIDMapping) string {
	return fmt.Sprintf("%d:%d:%d", mapping.ContainerID, mapping.HostID, mapping.Size)
}

func (p *ExternalImageManager) readEnvVars(imagePath string) ([]string, error) {
	imageConfigFile, err := os.Open(filepath.Join(imagePath, "image.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}

		return nil, fmt.Errorf("could not open image configuration: %s", err)
	}

	var imageConfig Image
	if err := json.NewDecoder(imageConfigFile).Decode(&imageConfig); err != nil {
		return nil, fmt.Errorf("parsing image config: %s", err)
	}

	return imageConfig.Config.Env, nil
}
