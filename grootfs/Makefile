.PHONY: all \
	test \
	go-vet concourse-go-vet go-generate \
	image push-image \
	update-deps unit integration

prefix ?= ./

ifdef STATIC_BINARY
	LDFLAG=--ldflags '-linkmode external -extldflags "-static"'
else
	LDFLAG=
endif

all: grootfs tardis

grootfs:
	GOOS=linux go build ${LDFLAG} -mod vendor -o build/grootfs .

tardis:
	GOOS=linux go build ${LDFLAG} -mod vendor -o build/tardis ./store/filesystems/overlayxfs/tardis

cf: all
	GOOS=linux go build -mod vendor -tags cloudfoundry -o tardis ./store/filesystems/overlayxfs/tardis

install:
	cp build/* $(prefix)

clean:
	rm -rf build/*

###### Help ###################################################################

help:
	@echo '    all ................................. builds the grootfs cli'
	@echo '    update-deps ......................... updates dependencies'
	@echo '    unit ................................ run unit tests'
	@echo '    integration ......................... run integration tests'
	@echo '    test ................................ runs tests in concourse-lite'
	@echo '    compile-tests ....................... checks that tests can be compiled'
	@echo '    go-vet .............................. runs go vet in grootfs source code'
	@echo '    concourse-go-vet .................... runs go vet in concourse-lite'
	@echo '    go-generate ......................... runs go generate in grootfs source code'
	@echo '    image ............................... builds a docker image'
	@echo '    push-image .......................... pushes image to docker-hub'

###### Dependencies ###########################################################

###### Testing ################################################################

compile-tests:
	ginkgo build -mod vendor -r .; find . -name '*.test' | xargs rm

unit:
	./script/test -u

unit-locally: go-vet
	./ci/scripts/unit-tests.sh

integration:
	./script/test -i

integration-locally:
	./ci/scripts/integration-tests.sh

test:
	./script/test -a

###### Go tools ###############################################################

go-vet:
	GOOS=linux go vet -mod vendor `go list -mod vendor ./... | grep -v vendor`

concourse-go-vet:
	fly -t runtime-garden e -c ci/tasks/go-vet.yml -i grootfs-git-repo=${PWD}

go-generate:
	GOOS=linux go generate `go list ./... | grep -v vendor`

###### Docker #################################################################

image:
	docker build -t cfgarden/grootfs-ci .

push-image:
	docker push cfgarden/grootfs-ci
