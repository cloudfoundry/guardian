module code.cloudfoundry.org/guardian

go 1.23.0

toolchain go1.23.6

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/jessevdk/go-flags => github.com/jessevdk/go-flags v1.4.0
)

require (
	code.cloudfoundry.org/archiver v0.28.0
	code.cloudfoundry.org/clock v1.30.0
	code.cloudfoundry.org/commandrunner v0.31.0
	code.cloudfoundry.org/debugserver v0.41.0
	code.cloudfoundry.org/garden v0.0.0-20250312022458-1d757121fb8a
	code.cloudfoundry.org/grootfs v0.30.0
	code.cloudfoundry.org/idmapper v0.0.0-20250305022537-ddc80fe13e43
	code.cloudfoundry.org/lager/v3 v3.29.0
	code.cloudfoundry.org/localip v0.33.0
	github.com/BurntSushi/toml v1.4.0
	github.com/cloudfoundry/dropsonde v1.1.0
	github.com/cloudfoundry/gosigar v1.3.89
	github.com/containerd/cgroups/v3 v3.0.5
	github.com/containerd/containerd/api v1.8.0
	github.com/containerd/containerd/v2 v2.0.3
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/typeurl/v2 v2.2.3
	github.com/eapache/go-resiliency v1.7.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jessevdk/go-flags v1.6.1
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.8.1
	github.com/mitchellh/copystructure v1.2.0
	github.com/moby/sys/reexec v0.1.0
	github.com/moby/sys/user v0.3.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.23.0
	github.com/onsi/gomega v1.36.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runc v1.2.5
	github.com/opencontainers/runtime-spec v1.2.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/pflag v1.0.6
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/urfave/cli/v2 v2.27.6
	github.com/vishvananda/netlink v1.3.0
	github.com/vishvananda/netns v0.0.5
	golang.org/x/sys v0.31.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20231105174938-2b5cbb29f3e2 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.12.9 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/checkpoint-restore/go-criu/v6 v6.3.0 // indirect
	github.com/cilium/ebpf v0.17.3 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20250217093333-7351ffed9384 // indirect
	github.com/containerd/console v1.0.4 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-runc v1.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.1 // indirect
	github.com/containerd/plugin v1.0.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20250302191652-9094ed2288e7 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/mrunalp/fileutils v0.5.1 // indirect
	github.com/opencontainers/selinux v1.11.1 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tedsuo/rata v1.0.0 // indirect
	github.com/urfave/cli v1.22.16 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250313205543-e70fdf4c4cb4 // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
