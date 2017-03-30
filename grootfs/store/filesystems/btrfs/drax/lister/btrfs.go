package lister // import "code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax/lister"

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"code.cloudfoundry.org/commandrunner"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

type BtrfsLister struct {
	commandRunner commandrunner.CommandRunner
	btrfsBin      string
}

func NewBtrfsLister(btrfsBin string, commandRunner commandrunner.CommandRunner) *BtrfsLister {
	return &BtrfsLister{
		commandRunner: commandRunner,
		btrfsBin:      btrfsBin,
	}
}

func (l *BtrfsLister) List(logger lager.Logger, imagePath string) ([]string, error) {
	logger = logger.Session("btrfs-listing", lager.Data{"imagePath": imagePath})
	logger.Info("starting")
	defer logger.Info("ending")

	cmd := exec.Command(l.btrfsBin, "subvolume", "list", imagePath)
	outputBuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outputBuffer
	errorBuffer := bytes.NewBuffer([]byte{})
	cmd.Stderr = errorBuffer

	if err := l.commandRunner.Run(cmd); err != nil {
		logger.Error("command-failed", err)
		return nil, errorspkg.Errorf("list subvolumes: %s: %s",
			strings.TrimSpace(outputBuffer.String()),
			strings.TrimSpace(errorBuffer.String()))
	}

	mountPoint, err := l.findMountPoint(imagePath)
	if err != nil {
		logger.Error("finding-mount-point-failed", err)
		return nil, errorspkg.Wrap(err, "find mount point")
	}

	volumes, err := l.extractPaths(outputBuffer, mountPoint, imagePath)
	if err != nil {
		logger.Error("parsing-list-output-failed", err)
		return nil, errorspkg.Wrap(err, "parse subvolume list")
	}

	sort.Sort(sort.Reverse(sort.StringSlice(volumes)))
	return volumes, nil
}

func (l *BtrfsLister) extractPaths(output io.Reader, mountPoint, path string) ([]string, error) {
	reader := bufio.NewReader(output)
	subvolumes := []string{}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, errorspkg.Wrap(err, "read btrfs list output")
		}

		if err != nil && line == "" {
			break
		}

		subvolumePaths := strings.Split(strings.TrimSpace(line), " ")
		if len(subvolumePaths) != 9 {
			return nil, errorspkg.Errorf("invalid output: %s", line)
		}

		absolutePath := filepath.Join(mountPoint, strings.TrimSpace(subvolumePaths[8]))
		if strings.HasPrefix(absolutePath, path) {
			subvolumes = append(subvolumes, absolutePath)
		}

		if err != nil {
			break
		}
	}

	return subvolumes, nil
}

func (l *BtrfsLister) findMountPoint(path string) (string, error) {
	contents, err := ioutil.ReadFile("/proc/self/mounts")
	if err != nil {
		return "", errorspkg.Wrap(err, "reading mounts")
	}

	for path != "/" {
		if strings.Contains(string(contents), path) {
			return path, nil
		}

		path = filepath.Dir(path)
	}

	return "", errorspkg.New("can't find mount point")
}

var basePathRegexp = regexp.MustCompile("(.*)/images/[^/]*")

func findStorePath(imagePath string) string {
	matches := basePathRegexp.FindStringSubmatch(imagePath)
	return matches[0]
}
