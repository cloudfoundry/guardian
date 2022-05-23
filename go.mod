module code.cloudfoundry.org/guardian

go 1.16

require (
	code.cloudfoundry.org/archiver v0.0.0-20210609160716-67523bd33dbf
	code.cloudfoundry.org/clock v1.0.0
	code.cloudfoundry.org/commandrunner v0.0.0-20180212143422-501fd662150b
	code.cloudfoundry.org/debugserver v0.0.0-20210608171006-d7658ce493f4
	code.cloudfoundry.org/garden v0.0.0-20210608104724-fa3a10d59c82
	code.cloudfoundry.org/grootfs v0.30.0
	code.cloudfoundry.org/idmapper v0.0.0-20210608104755-adcde2231d2c
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/localip v0.0.0-20210608161955-43c3ec713c20
	github.com/BurntSushi/toml v1.1.0
	github.com/bits-and-blooms/bitset v1.2.1 // indirect
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosigar v1.3.2
	github.com/containerd/containerd v1.5.9
	github.com/containerd/typeurl v1.0.2
	github.com/docker/docker v20.10.13+incompatible
	github.com/eapache/go-resiliency v1.2.0
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jessevdk/go-flags v1.5.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/mitchellh/copystructure v1.2.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202193544-a5463b7f9c84
	github.com/opencontainers/runc v1.1.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/st3v/glager v0.3.0
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00
	github.com/urfave/cli/v2 v2.8.0
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	golang.org/x/sys v0.0.0-20220310020820-b874c991c1a5
	gopkg.in/yaml.v2 v2.4.0
)

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/cloudfoundry/gosigar => github.com/cloudfoundry/gosigar v1.1.0
	github.com/docker/distribution => github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.13+incompatible
	github.com/jessevdk/go-flags => github.com/jessevdk/go-flags v1.4.0
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.0
	github.com/opencontainers/selinux => github.com/opencontainers/selinux v1.8.2
	golang.org/x/text => golang.org/x/text v0.3.7
)
