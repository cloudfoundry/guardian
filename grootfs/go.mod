module code.cloudfoundry.org/grootfs

go 1.12

require (
	code.cloudfoundry.org/commandrunner v0.0.0-20180212143422-501fd662150b
	code.cloudfoundry.org/idmapper v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/lager v0.0.0-20181119165122-baf208c4c56b
	github.com/DataDog/zstd v1.4.0 // indirect
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/apoydence/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973 // indirect
	github.com/checkpoint-restore/go-criu v0.0.0-20191125063657-fcdcd07065c5 // indirect
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cloudfoundry/gosteno v0.0.0-20150423193413-0c8581caea35 // indirect
	github.com/cloudfoundry/loggregatorlib v0.0.0-20170823162133-36eddf15ef12 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20171206171820-b33733203bb4
	github.com/containers/image v1.5.1
	github.com/containers/storage v1.12.13
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v0.7.3-0.20190329212828-d7ab8ad145fa
	github.com/docker/docker-credential-helpers v0.6.0 // indirect
	github.com/docker/go-connections v0.3.0 // indirect
	github.com/docker/go-metrics v0.0.0-20180209012529-399ea8c73916 // indirect
	github.com/docker/go-units v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2 // indirect
	github.com/klauspost/compress v1.9.8 // indirect
	github.com/klauspost/cpuid v1.2.1 // indirect
	github.com/klauspost/pgzip v1.2.1 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mailru/easyjson v0.0.0-20180323154445-8b799c424f57 // indirect
	github.com/mattn/go-shellwords v1.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mistifyio/go-zfs v2.1.1+incompatible // indirect
	github.com/mrunalp/fileutils v0.0.0-20171103030105-7d4729fb3618 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc10
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/opencontainers/selinux v1.3.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/poy/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7 // indirect
	github.com/prometheus/client_golang v0.0.0-20180529170124-42bc0a18c220 // indirect
	github.com/prometheus/client_model v0.0.0-20171117100541-99fa1f4be8e5 // indirect
	github.com/prometheus/common v0.0.0-20180518154759-7600349dcfe1 // indirect
	github.com/seccomp/libseccomp-golang v0.9.1 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/st3v/glager v0.3.0
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/tscolari/lagregator v0.0.0-20161103133944-b0fb43b01861
	github.com/urfave/cli/v2 v2.1.1
	github.com/vbatts/tar-split v0.11.1 // indirect
	github.com/ventu-io/go-shortid v0.0.0-20160104014424-6c56cef5189c
	github.com/vishvananda/netlink v1.1.0 // indirect
	golang.org/x/sys v0.0.0-20200202164722-d101bd2416d5
	gopkg.in/yaml.v2 v2.2.8
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
)

replace code.cloudfoundry.org/idmapper => ../idmapper
