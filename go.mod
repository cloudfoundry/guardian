module code.cloudfoundry.org/guardian

go 1.24.9

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/cilium/ebpf => github.com/cilium/ebpf v0.17.3
	github.com/jessevdk/go-flags => github.com/jessevdk/go-flags v1.4.0
)

require (
	code.cloudfoundry.org/archiver v0.58.0
	code.cloudfoundry.org/clock v1.57.0
	code.cloudfoundry.org/commandrunner v0.54.0
	code.cloudfoundry.org/debugserver v0.80.0
	code.cloudfoundry.org/garden v0.0.0-20260107023026-c1405e3f9a7c
	code.cloudfoundry.org/grootfs v0.30.0
	code.cloudfoundry.org/idmapper v0.0.0-20260107023157-ab2fdcb33dbd
	code.cloudfoundry.org/lager/v3 v3.57.0
	code.cloudfoundry.org/localip v0.59.0
	github.com/BurntSushi/toml v1.6.0
	github.com/cloudfoundry/dropsonde v1.1.0
	github.com/cloudfoundry/gosigar v1.3.112
	github.com/containerd/cgroups/v3 v3.1.2
	github.com/containerd/containerd/api v1.10.0
	github.com/containerd/containerd/v2 v2.2.1
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/typeurl/v2 v2.2.3
	github.com/eapache/go-resiliency v1.7.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jessevdk/go-flags v1.6.1
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.8.1
	github.com/mitchellh/copystructure v1.2.0
	github.com/moby/sys/reexec v0.1.0
	github.com/moby/sys/user v0.4.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.27.4
	github.com/onsi/gomega v1.39.0
	github.com/opencontainers/cgroups v0.0.6
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runc v1.4.0
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/pflag v1.0.10
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/urfave/cli/v2 v2.27.7
	github.com/vishvananda/netlink v1.3.1
	github.com/vishvananda/netns v0.0.5
	golang.org/x/sys v0.40.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	cyphar.com/go-pathrs v0.2.2 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.14.0-rc.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/checkpoint-restore/go-criu/v7 v7.2.0 // indirect
	github.com/cilium/ebpf v0.20.0 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20251217143644-d1670a435f2a // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-runc v1.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.2 // indirect
	github.com/containerd/plugin v1.0.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/coreos/go-systemd/v22 v22.6.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260106004452-d7df1bf2cac7 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/mrunalp/fileutils v0.5.1 // indirect
	github.com/opencontainers/selinux v1.13.1 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/seccomp/libseccomp-golang v0.11.1 // indirect
	github.com/tedsuo/rata v1.0.0 // indirect
	github.com/urfave/cli v1.22.17 // indirect
	github.com/xrash/smetrics v0.0.0-20250705151800-55b8f293f342 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.64.0 // indirect
	go.opentelemetry.io/otel v1.39.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
