package runner

import (
	"io"
	"io/ioutil"
	"net"
	"os"
	"syscall"
	"time"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/lager"
)

///////////////////////////////////////////////////////////////////////////////
// Registry options
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithCredentials(username, password string) Runner {
	nr := r
	nr.RegistryUsername = username
	nr.RegistryPassword = password
	return nr
}

func (r Runner) WithInsecureRegistry(registry string) Runner {
	nr := r
	nr.InsecureRegistry = registry
	return nr
}

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

func (r Runner) WithoutDriver() Runner {
	nr := r
	nr.Driver = ""
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

func (r Runner) WithXFSProgsPath(xfsProgsPath string) Runner {
	nr := r
	nr.XFSProgsPath = xfsProgsPath
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

func (r Runner) WithConfig(configPath string) Runner {
	nr := r
	nr.ConfigPath = configPath
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Timeout
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithTimeout(timeout time.Duration) Runner {
	nr := r
	nr.Timeout = timeout
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Env Variables
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithEnvVar(variable string) Runner {
	nr := r
	nr.EnvVars = append(nr.EnvVars, variable)
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// Clean on Start
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithClean() Runner {
	nr := r
	nr.CleanOnCreate = true
	return nr
}

func (r Runner) WithNoClean() Runner {
	nr := r
	nr.NoCleanOnCreate = true
	return nr
}

///////////////////////////////////////////////////////////////////////////////
// SysProcAttributes
///////////////////////////////////////////////////////////////////////////////

func (r Runner) RunningAsUser(uid, gid uint32) Runner {
	nr := r
	nr.SysCredential = &syscall.Credential{
		Uid: uid,
		Gid: gid,
	}
	return nr
}
