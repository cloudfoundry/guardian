package runrunc

import (
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/goci"
)

func envWithDefaultPath(defaultPath string, env []string) []string {
	for _, envVar := range env {
		if strings.Contains(envVar, "PATH=") {
			return env
		}
	}

	return append(env, defaultPath)
}

func envWithUser(env []string, user string) []string {
	for _, envVar := range env {
		if strings.Contains(envVar, "USER=") {
			return env
		}
	}

	if user != "" {
		return append(env, "USER="+user)
	} else {
		return append(env, "USER=root")
	}
}

func envFor(uid int, bndl goci.Bndl, spec garden.ProcessSpec) []string {
	requestedEnv := append(bndl.Spec.Process.Env, spec.Env...)

	defaultPath := DefaultPath
	if uid == 0 {
		defaultPath = DefaultRootPath
	}

	return envWithUser(envWithDefaultPath(defaultPath, requestedEnv), spec.User)
}
