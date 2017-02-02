package runner

import (
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/lager"
)

///////////////////////////////////////////////////////////////////////////////
// Store path
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithStore(path string) Runner {
	nr := r
	nr.StorePath = path
	return nr
}

func (r Runner) WithoutStore() Runner {
	nr := r
	nr.StorePath = ""
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Drivers
///////////////////////////////////////////////////////////////////////////////
func (r Runner) WithDriver(driver string) Runner {
	nr := r
	nr.Driver = driver
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Binaries
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithDraxBin(draxBin string) Runner {
	nr := r
	nr.DraxBin = draxBin
	return nr
}

func (r Runner) WithoutDraxBin() Runner {
	nr := r
	nr.DraxBin = ""
	return nr
}

func (r Runner) WithBtrfsBin(btrfsBin string) Runner {
	nr := r
	nr.BtrfsBin = btrfsBin
	return nr
}

func (r Runner) WithNewuidmapBin(newuidmapBin string) Runner {
	nr := r
	nr.NewuidmapBin = newuidmapBin
	return nr
}

func (r Runner) WithNewgidmapBin(newgidmapBin string) Runner {
	nr := r
	nr.NewgidmapBin = newgidmapBin
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Metrics
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithMetronEndpoint(host net.IP, port uint16) Runner {
	nr := r
	nr.MetronHost = host
	nr.MetronPort = port
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Logging
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithLogLevel(level lager.LogLevel) Runner {
	nr := r
	nr.LogLevel = level
	nr.LogLevelSet = true
	return nr
}

func (r Runner) WithoutLogLevel() Runner {
	nr := r
	nr.LogLevelSet = false
	return nr
}

func (r Runner) WithLogFile(path string) Runner {
	nr := r
	nr.LogFile = path
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Streams
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithStdout(stdout io.Writer) Runner {
	nr := r
	nr.Stdout = stdout
	return nr
}

func (r Runner) WithStderr(stderr io.Writer) Runner {
	nr := r
	nr.Stderr = stderr
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Configuration file
///////////////////////////////////////////////////////////////////////////////

func (r *Runner) SetConfig(cfg config.Config) error {
	configYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	configFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer configFile.Close()

	_, err = configFile.Write(configYaml)
	if err != nil {
		os.Remove(configFile.Name())
		return err
	}

	r.ConfigPath = configFile.Name()

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Timeout
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithTimeout(timeout time.Duration) Runner {
	nr := r
	nr.Timeout = timeout
	return nr
}
