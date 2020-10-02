module code.cloudfoundry.org/guardian

go 1.14

require (
	code.cloudfoundry.org/archiver v0.0.0-20180525162158-e135af3d5a2a
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c
	code.cloudfoundry.org/commandrunner v0.0.0-20180212143422-501fd662150b
	code.cloudfoundry.org/debugserver v0.0.0-20170501225606-70715da12ee9
	code.cloudfoundry.org/garden v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/grootfs v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/idmapper v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/localip v0.0.0-20170223024724-b88ad0dea95c
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/hcsshim v0.8.10 // indirect
	github.com/Microsoft/hcsshim/test v0.0.0-20201001234239-936eeeb286fd // indirect
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/containerd/cgroups v0.0.0-20200824123100-0b889c03f102 // indirect
	github.com/containerd/console v1.0.0 // indirect
	github.com/containerd/containerd v1.4.1
	github.com/containerd/fifo v0.0.0-20200410184934-f15a3290365b // indirect
	github.com/containerd/go-runc v0.0.0-20200707131846-23d84c510c41 // indirect
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.1
	github.com/coreos/go-systemd/v22 v22.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/docker v0.7.3-0.20190329212828-d7ab8ad145fa
	github.com/eapache/go-resiliency v1.2.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1
	github.com/mailru/easyjson v0.0.0-20190403194419-1ea4449da983 // indirect
	github.com/mitchellh/copystructure v0.0.0-20170525013902-d23ffcb85de3
	github.com/mitchellh/reflectwalk v0.0.0-20170726202117-63d60e9d0dbc // indirect
	github.com/mrunalp/fileutils v0.5.0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc92
	github.com/opencontainers/runtime-spec v1.0.3-0.20200520003142-237cc4f519e2
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/seccomp/libseccomp-golang v0.9.1 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/st3v/glager v0.3.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tedsuo/ifrit v0.0.0-20180410193936-e89a512c3162
	github.com/urfave/cli/v2 v2.2.0
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	github.com/willf/bitset v1.1.11 // indirect
	go.opencensus.io v0.22.4 // indirect
	golang.org/x/sync v0.0.0-20200930132711-30421366ff76 // indirect
	golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f
	google.golang.org/genproto v0.0.0-20201001141541-efaab9d3c4f7 // indirect
	google.golang.org/grpc v1.32.0 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gotest.tools/v3 v3.0.2 // indirect
)

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20190205005809-0d3efadf0154
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc90
)
