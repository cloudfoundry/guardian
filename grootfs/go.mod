module code.cloudfoundry.org/grootfs

go 1.16

require (
	code.cloudfoundry.org/commandrunner v0.0.0-20180212143422-501fd662150b
	code.cloudfoundry.org/idmapper v0.0.0-20210608104755-adcde2231d2c
	code.cloudfoundry.org/lager v0.0.0-20181119165122-baf208c4c56b
	github.com/apoydence/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosteno v0.0.0-20150423193413-0c8581caea35 // indirect
	github.com/cloudfoundry/loggregatorlib v0.0.0-20170823162133-36eddf15ef12 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20200416163440-a42463ba266b
	github.com/containers/image/v5 v5.22.0
	github.com/containers/storage v1.42.0
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.22+incompatible
	github.com/docker/go-units v0.5.0
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.19.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/opencontainers/runc v1.1.3
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.0
	github.com/st3v/glager v0.3.0
	github.com/teris-io/shortid v0.0.0-20201117134242-e59966efd125 // indirect
	github.com/tscolari/lagregator v0.0.0-20161103133944-b0fb43b01861
	github.com/urfave/cli/v2 v2.3.0
	github.com/ventu-io/go-shortid v0.0.0-20201117134242-e59966efd125
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
	gopkg.in/yaml.v2 v2.4.0
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
)

replace (
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/docker/distribution => github.com/docker/distribution v2.8.1+incompatible
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.0
	golang.org/x/text => golang.org/x/text v0.3.7
)
