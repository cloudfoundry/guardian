package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"code.cloudfoundry.org/guardian/guardiancmd"

	"github.com/containerd/containerd"
	containers "github.com/containerd/containerd/api/services/containers/v1"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/cmd/containerd/command"
	// _ "github.com/containerd/containerd/diff/walking/plugin"
	// _ "github.com/containerd/containerd/gc/scheduler"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plugin"
	// _ "github.com/containerd/containerd/runtime/v1"
	_ "github.com/containerd/containerd/runtime/v1/linux"
	"github.com/containerd/containerd/services"
	_ "github.com/containerd/containerd/services/containers"
	_ "github.com/containerd/containerd/snapshots/native"
	// _ "github.com/containerd/containerd/services/content"
	// _ "github.com/containerd/containerd/services/diff"
	// _ "github.com/containerd/containerd/services/events"
	// _ "github.com/containerd/containerd/services/healthcheck"
	// _ "github.com/containerd/containerd/services/images"
	// _ "github.com/containerd/containerd/services/introspection"
	// _ "github.com/containerd/containerd/services/leases"
	// _ "github.com/containerd/containerd/services/namespaces"
	// _ "github.com/containerd/containerd/services/snapshots"
	_ "github.com/containerd/containerd/services/tasks"
	// _ "github.com/containerd/containerd/services/version"

	flags "github.com/jessevdk/go-flags"
)

// version is overwritten at compile time by passing
// -ldflags -X main.version=<version>
var version = "dev"

func init() {
	rand.Seed(time.Now().UnixNano())
	plugin.Register(&plugin.Registration{
		Type: plugin.GRPCPlugin,
		ID:   "garden",
		// Config: &config,
		Requires: []plugin.Type{
			plugin.ServicePlugin,
			plugin.RuntimePlugin,
		},
		InitFn: initContainerdClient,
	})
}

func initContainerdClient(ic *plugin.InitContext) (interface{}, error) {
	servicesOpts, err := getContainerdServicesOpts(ic)
	if err != nil {
		return nil, fmt.Errorf("failed to get services %s", err)
	}

	log.G(ic.Context).Info("Connect containerd service")
	guardiancmd.ContainerdClient, err = containerd.New(
		"",
		containerd.WithDefaultNamespace("garden"),
		containerd.WithDefaultRuntime(plugin.RuntimeLinuxV1),
		containerd.WithServices(servicesOpts...),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client %s", err)
	}

	return nil, nil
}

func getContainerdServicesOpts(ic *plugin.InitContext) ([]containerd.ServicesOpt, error) {
	plugins, err := ic.GetByType(plugin.ServicePlugin)
	if err != nil {
		return nil, fmt.Errorf("failed to get service plugin: %v", err)
	}

	opts := []containerd.ServicesOpt{
		containerd.WithEventService(ic.Events),
	}
	for s, fn := range map[string]func(interface{}) containerd.ServicesOpt{
		// services.ContentService: func(s interface{}) containerd.ServicesOpt {
		// 	return containerd.WithContentStore(s.(content.Store))
		// },
		// services.ImagesService: func(s interface{}) containerd.ServicesOpt {
		// 	return containerd.WithImageService(s.(images.ImagesClient))
		// },
		// services.SnapshotsService: func(s interface{}) containerd.ServicesOpt {
		// 	return containerd.WithSnapshotters(s.(map[string]snapshots.Snapshotter))
		// },
		services.ContainersService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithContainerService(s.(containers.ContainersClient))
		},
		services.TasksService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithTaskService(s.(tasks.TasksClient))
		},
		// services.DiffService: func(s interface{}) containerd.ServicesOpt {
		// 	return containerd.WithDiffService(s.(diff.DiffClient))
		// },
		// services.NamespacesService: func(s interface{}) containerd.ServicesOpt {
		// 	return containerd.WithNamespaceService(s.(namespaces.NamespacesClient))
		// },
		// services.LeasesService: func(s interface{}) containerd.ServicesOpt {
		// 	return containerd.WithLeasesService(s.(leases.Manager))
		// },
	} {
		p := plugins[s]
		if p == nil {
			return nil, fmt.Errorf("service %q not found", s)
		}
		i, err := p.Instance()
		if err != nil {
			return nil, fmt.Errorf("failed to get instance of service %q: %v", s, err)
		}
		if i == nil {
			return nil, fmt.Errorf("instance of service %q not found", s)
		}
		opts = append(opts, fn(i))
	}
	return opts, nil
}

func main() {
	configFilePath := flag.String("config", "", "config file path")
	printVersion := flag.Bool("v", false, "print version")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	errChan := make(chan error)
	go func() {
		err := <-errChan
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}()

	runContainerd(errChan)
	for {
		if guardiancmd.ContainerdClient != nil {
			break
		}
		time.Sleep(time.Second)
	}
	runGarden(configFilePath, errChan)
}

func runGarden(configFilePath *string, errChan chan<- error) {
	cmd := &guardiancmd.GdnCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	if *configFilePath != "" {
		iniParser := flags.NewIniParser(parser)
		must(iniParser.ParseFile(*configFilePath))
	}

	_, err := parser.Parse()
	if err != nil {
		errChan <- err
	}
}

func runContainerd(errChan chan<- error) {
	args := []string{"pesho", "-c", "/var/vcap/jobs/garden/config/containerd.toml"}
	go func() {
		app := command.App()
		if err := app.Run(args); err != nil {
			errChan <- err
		}
	}()
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var mustNot = must
