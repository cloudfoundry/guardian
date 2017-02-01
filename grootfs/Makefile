.PHONY: all \
	test concourse-groot-test concourse-root-test concourse-test \
	go-vet concourse-go-vet go-generate \
	image push-image \
	update-deps

all:
	GOOS=linux go build -o grootfs .
	GOOS=linux go build -o drax ./store/filesystems/btrfs/drax

###### Help ###################################################################

help:
	@echo '    all ................................. builds the grootfs cli'
	@echo '    deps ................................ installs dependencies'
	@echo '    update-deps ......................... updates dependencies'
	@echo '    test ................................ runs tests locally'
	@echo '    concourse-test ...................... runs tests in concourse-lite'
	@echo '    compile-tests ....................... checks that tests can be compiled'
	@echo '    go-vet .............................. runs go vet in grootfs source code'
	@echo '    concourse-go-vet .................... runs go vet in concourse-lite'
	@echo '    go-generate ......................... runs go generate in grootfs source code'
	@echo '    image ............................... builds a docker image'
	@echo '    push-image .......................... pushes image to docker-hub'

###### Dependencies ###########################################################

deps:
	git submodule update --init --recursive

update-deps:
	echo "coming soon"

###### Testing ################################################################

compile-tests:
	ginkgo build -r .; find . -name '*.test' | xargs rm

test:
	ginkgo -r -p -race -skipPackage integration .

concourse-test: go-vet
	./hack/run-tests -r -g "-p"

###### Go tools ###############################################################

go-vet:
	GOOS=linux go vet `go list ./... | grep -v vendor`

concourse-go-vet:
	fly -t lite e -x -c ci/tasks/go-vet.yml -i grootfs-git-repo=${PWD}

go-generate:
	GOOS=linux go generate `go list ./... | grep -v vendor`

###### Docker #################################################################

image:
	docker build -t cfgarden/grootfs-ci .

push-image:
	docker push cfgarden/grootfs-ci
