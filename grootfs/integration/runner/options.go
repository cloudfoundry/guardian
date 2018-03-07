package runner

import (
	"io"
	"io/ioutil"
	"net"
	"os"
	"syscall"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/lager"
)

///////////////////////////////////////////////////////////////////////////////
// Registry options
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithCredentials(username, password string) Runner {
	r.RegistryUsername = username
	r.RegistryPassword = password
	return r
}

func (r Runner) WithInsecureRegistry(registry string) Runner {
	r.InsecureRegistry = registry
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Store path
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithStore(path string) Runner {
	r.StorePath = path
	return r
}

func (r Runner) WithoutStore() Runner {
	r.StorePath = ""
	return r
}

func (r Runner) SkipInitStore() Runner {
	r.skipInitStore = true
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Drivers
///////////////////////////////////////////////////////////////////////////////
func (r Runner) WithDriver(driver string) Runner {
	r.Driver = driver
	return r
}

func (r Runner) WithoutDriver() Runner {
	r.Driver = ""
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Binaries
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithTardisBin(tardisBin string) Runner {
	r.TardisBin = tardisBin
	return r
}

func (r Runner) WithoutTardisBin() Runner {
	r.TardisBin = ""
	return r
}

func (r Runner) WithNewuidmapBin(newuidmapBin string) Runner {
	r.NewuidmapBin = newuidmapBin
	return r
}

func (r Runner) WithNewgidmapBin(newgidmapBin string) Runner {
	r.NewgidmapBin = newgidmapBin
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Metrics
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithMetronEndpoint(host net.IP, port uint16) Runner {
	r.MetronHost = host
	r.MetronPort = port
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Logging
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithLogLevel(level lager.LogLevel) Runner {
	r.LogLevel = level
	r.LogLevelSet = true
	return r
}

func (r Runner) WithoutLogLevel() Runner {
	r.LogLevelSet = false
	return r
}

func (r Runner) WithLogFile(path string) Runner {
	r.LogFile = path
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Streams
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithStdout(stdout io.Writer) Runner {
	r.Stdout = stdout
	return r
}

func (r Runner) WithStderr(stderr io.Writer) Runner {
	r.Stderr = stderr
	return r
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
		_ = os.Remove(configFile.Name())
		return err
	}

	if err := os.Chmod(configFile.Name(), 0666); err != nil {
		return err
	}

	r.ConfigPath = configFile.Name()

	return nil
}

func (r Runner) WithConfig(configPath string) Runner {
	r.ConfigPath = configPath
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Env Variables
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithEnvVar(variable string) Runner {
	r.EnvVars = append(r.EnvVars, variable)
	return r
}

///////////////////////////////////////////////////////////////////////////////
// Clean on Start
///////////////////////////////////////////////////////////////////////////////

func (r Runner) WithClean() Runner {
	r.CleanOnCreate = true
	return r
}

func (r Runner) WithNoClean() Runner {
	r.NoCleanOnCreate = true
	return r
}

///////////////////////////////////////////////////////////////////////////////
// SysProcAttributes
///////////////////////////////////////////////////////////////////////////////

func (r Runner) RunningAsUser(uid, gid int) Runner {
	r.SysCredential = syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}
	return r
}

///////////////////////////////////////////////////////////////////////////////
// OCI Checksum Validation
///////////////////////////////////////////////////////////////////////////////

func (r Runner) SkipLayerCheckSumValidation() Runner {
	r.SkipLayerValidation = true
	return r
}
