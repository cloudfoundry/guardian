module code.cloudfoundry.org/grootfs

go 1.12

require (
	code.cloudfoundry.org/commandrunner v0.0.0-20180212143422-501fd662150b
	code.cloudfoundry.org/idmapper v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/lager v0.0.0-20181119165122-baf208c4c56b
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Microsoft/go-winio v0.4.7 // indirect
	github.com/apoydence/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973 // indirect
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosteno v0.0.0-20150423193413-0c8581caea35 // indirect
	github.com/cloudfoundry/loggregatorlib v0.0.0-20170823162133-36eddf15ef12 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20171206171820-b33733203bb4
	github.com/containerd/continuity v0.0.0-20180524210603-d3c23511c1bf // indirect
	github.com/containers/image v1.5.1
	github.com/containers/storage v1.12.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v0.7.3-0.20190329212828-d7ab8ad145fa
	github.com/docker/docker-credential-helpers v0.6.0 // indirect
	github.com/docker/go-connections v0.3.0 // indirect
	github.com/docker/go-metrics v0.0.0-20180209012529-399ea8c73916 // indirect
	github.com/docker/go-units v0.3.3
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gogo/protobuf v1.0.0 // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mailru/easyjson v0.0.0-20180323154445-8b799c424f57 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc7
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/pkg/errors v0.8.1
	github.com/poy/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/prometheus/client_golang v0.0.0-20180529170124-42bc0a18c220 // indirect
	github.com/prometheus/client_model v0.0.0-20171117100541-99fa1f4be8e5 // indirect
	github.com/prometheus/common v0.0.0-20180518154759-7600349dcfe1 // indirect
	github.com/prometheus/procfs v0.0.0-20180528183257-fe93d378a6b0 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/st3v/glager v0.3.0
	github.com/tscolari/lagregator v0.0.0-20161103133944-b0fb43b01861
	github.com/urfave/cli v0.0.0-20161006035353-55f715e28c46
	github.com/ventu-io/go-shortid v0.0.0-20160104014424-6c56cef5189c
	golang.org/x/net v0.0.0-20190603091049-60506f45cf65 // indirect
	golang.org/x/sys v0.0.0-20190602015325-4c4f7f33c9ed
	golang.org/x/text v0.3.2 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible // indirect
)

replace code.cloudfoundry.org/idmapper => ../idmapper
