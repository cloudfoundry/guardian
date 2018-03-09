package peas

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/peas/processwaiter"
	"code.cloudfoundry.org/lager"
	multierror "github.com/hashicorp/go-multierror"
)

type PeaCleaner struct {
	RuncDeleter    RuncDeleter
	Volumizer      Volumizer
	Waiter         processwaiter.ProcessWaiter
	DepotDirectory string
}

func NewPeaCleaner(runcDeleter RuncDeleter, volumizer Volumizer, depotDir string) gardener.PeaCleaner {
	return &PeaCleaner{
		RuncDeleter:    runcDeleter,
		Volumizer:      volumizer,
		Waiter:         processwaiter.WaitOnProcess,
		DepotDirectory: depotDir,
	}
}

func (p *PeaCleaner) Clean(log lager.Logger, handle string) error {
	log = log.Session("clean-pea", lager.Data{"peaHandle": handle})
	log.Info("start")
	defer log.Info("end")

	var result *multierror.Error
	err := p.RuncDeleter.Delete(log, true, handle)
	if err != nil {
		result = multierror.Append(result, err)
	}
	err = p.Volumizer.Destroy(log, handle)
	if err != nil {
		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
}

func (p *PeaCleaner) CleanAll(log lager.Logger) error {
	log = log.Session("clean-all-peas")
	log.Info("start")
	defer log.Info("end")

	peas, err := p.getPeas()
	if err != nil {
		return err
	}

	for _, pea := range peas {
		go func(pea Pea) {
			log.Info("pea-cleaner-goroutine-started", lager.Data{"pea": pea})
			defer log.Info("pea-cleaner-goroutine-ended", lager.Data{"pea": pea})
			if err := p.Waiter.Wait(pea.Pid); err != nil {
				log.Error("error-waiting-on-pea", err, lager.Data{"pea": pea})
			}

			if err := p.Clean(log, pea.Handle); err != nil {
				log.Error("error-cleaning-up-pea", err, lager.Data{"pea": pea})
				return
			}
		}(pea)
	}

	return nil
}

func (p *PeaCleaner) getPeas() ([]Pea, error) {
	peas := []Pea{}

	processDirs, err := getProcessDirs(p.DepotDirectory)
	if err != nil {
		return nil, err
	}

	peaDirs, err := filterStringSlice(processDirs, isBundle)
	if err != nil {
		return nil, err
	}

	for _, dir := range peaDirs {
		pid, err := readPidfile(filepath.Join(dir, "pidfile"))
		if err != nil {
			return nil, err
		}
		peas = append(peas, Pea{filepath.Base(dir), pid})
	}
	return peas, nil
}

type Pea struct {
	Handle string
	Pid    int
}

func isBundle(path string) (bool, error) {
	return fileExists(filepath.Join(path, "config.json"))
}

func getProcessDirs(depotDirectory string) ([]string, error) {
	var processes []string

	paths, err := readDirPaths(depotDirectory)
	if err != nil {
		return nil, err
	}

	for _, path := range paths {
		processDirs, err := getProcessDirsForBundle(path)
		if err != nil {
			return nil, err
		}

		processes = append(processes, processDirs...)
	}
	return processes, nil
}

func getProcessDirsForBundle(bundlePath string) ([]string, error) {
	paths, err := readDirPaths(filepath.Join(bundlePath, "processes"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) {
		return []string{}, nil
	}

	return paths, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func readDirPaths(path string) ([]string, error) {
	names := []string{}
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		names = append(names, filepath.Join(path, info.Name()))
	}
	return names, nil
}

func filterStringSlice(slice []string, filter func(string) (bool, error)) ([]string, error) {
	var filtered []string

	for _, item := range slice {
		include, err := filter(item)
		if err != nil {
			return nil, err
		}
		if include {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func readPidfile(path string) (int, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(bytes.TrimSpace(content)))
}
