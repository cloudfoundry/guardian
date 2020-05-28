package groot

import (
	"os"
	"os/exec"
	"strconv"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/lager"
)

type asyncCleaner struct {
	logFile            string
	logLevel           string
	logTimestampFormat string
	storePath          string
	metronEndpoint     string
	tardisBin          string
	newuidmapBin       string
	newgidmapBin       string
}

func YouAreCleaner(cfg config.Config) Cleaner {
	return &asyncCleaner{
		logFile:            cfg.Create.CleanLogFile,
		logLevel:           cfg.LogLevel,
		storePath:          cfg.StorePath,
		metronEndpoint:     cfg.MetronEndpoint,
		tardisBin:          cfg.TardisBin,
		newuidmapBin:       cfg.NewuidmapBin,
		newgidmapBin:       cfg.NewgidmapBin,
		logTimestampFormat: cfg.LogTimestampFormat,
	}
}

func (c *asyncCleaner) Clean(logger lager.Logger, cleanThresholdBytes int64) (bool, error) {
	cleanCommandArgs := []string{}
	useLogFile := c.logFile != ""
	if useLogFile {
		cleanCommandArgs = append(cleanCommandArgs, "--log-file", c.logFile)
	}
	if c.logLevel != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--log-level", c.logLevel)
	}
	if c.logTimestampFormat != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--log-timestamp-format", c.logTimestampFormat)
	}
	if c.storePath != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--store", c.storePath)
	}
	if c.metronEndpoint != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--metron-endpoint", c.metronEndpoint)
	}
	if c.tardisBin != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--tardis-bin", c.tardisBin)
	}
	if c.newuidmapBin != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--newuidmap-bin", c.newuidmapBin)
	}
	if c.newgidmapBin != "" {
		cleanCommandArgs = append(cleanCommandArgs, "--newgidmap-bin", c.newgidmapBin)
	}

	cleanCommandArgs = append(cleanCommandArgs, "clean", "--threshold-bytes", strconv.FormatInt(cleanThresholdBytes, 10))
	cleanCommand := exec.Command(os.Args[0], cleanCommandArgs...)
	if !useLogFile {
		cleanCommand.Stderr = os.Stderr
	}

	return true, cleanCommand.Start()
}
