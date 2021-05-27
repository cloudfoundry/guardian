module code.cloudfoundry.org/guardian

go 1.15

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
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosigar v1.1.0
	github.com/containerd/containerd v1.5.2
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.2
	github.com/docker/docker v0.7.3-0.20190329212828-d7ab8ad145fa
	github.com/eapache/go-resiliency v1.2.0
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1
	github.com/mitchellh/copystructure v0.0.0-20170525013902-d23ffcb85de3
	github.com/mitchellh/reflectwalk v0.0.0-20170726202117-63d60e9d0dbc // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc95
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/st3v/glager v0.3.0
	github.com/tedsuo/ifrit v0.0.0-20180410193936-e89a512c3162
	github.com/urfave/cli/v2 v2.3.0
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae
	go.opencensus.io v0.22.5 // indirect
	golang.org/x/sys v0.0.0-20210426230700-d19ff857e887
	google.golang.org/genproto v0.0.0-20201201144952-b05cb90ed32e // indirect
	google.golang.org/grpc v1.33.2 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20190205005809-0d3efadf0154
)
