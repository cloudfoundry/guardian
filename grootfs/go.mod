module code.cloudfoundry.org/grootfs

go 1.16

require (
	code.cloudfoundry.org/commandrunner v0.0.0-20180212143422-501fd662150b
	code.cloudfoundry.org/idmapper v0.0.0-20210208154102-be3abcaf6e2b
	code.cloudfoundry.org/lager v0.0.0-20181119165122-baf208c4c56b
	github.com/apoydence/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosteno v0.0.0-20150423193413-0c8581caea35 // indirect
	github.com/cloudfoundry/loggregatorlib v0.0.0-20170823162133-36eddf15ef12 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20200416163440-a42463ba266b
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v1.32.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.7+incompatible
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc95
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/pkg/errors v0.9.1
	github.com/poy/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/prometheus/client_golang v1.10.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/st3v/glager v0.3.0
	github.com/teris-io/shortid v0.0.0-20201117134242-e59966efd125 // indirect
	github.com/tscolari/lagregator v0.0.0-20161103133944-b0fb43b01861
	github.com/urfave/cli/v2 v2.3.0
	github.com/ventu-io/go-shortid v0.0.0-20201117134242-e59966efd125
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
)

replace (
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/containers/image => github.com/containers/image v1.5.1
)
