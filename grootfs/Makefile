.PHONY: all \
	test concourse-groot-test concourse-root-test concourse-test \
	go-vet \
	image push-image

all:
	GOOS=linux go build .

###### Help ###################################################################

help:
	@echo '    all ................................. builds the grootfs cli'
	@echo '    test ................................ runs tests locally'
	@echo '    concourse-test ...................... runs tests in concourse-lite'
	@echo '    concourse-groot-test ................ runs groot tests in concourse-lite'
	@echo '    concourse-root-test ................. runs root tests in concourse-lite'
	@echo '    go-vet .............................. runs go vet in grootfs source code'
	@echo '    go-generate ......................... runs go generate in grootfs source code'
	@echo '    image ............................... builds a docker image'
	@echo '    push-image .......................... pushes image to docker-hub'

###### Testing ################################################################

test:
	ginkgo -r -p -skipPackage integration .

concourse-groot-test:
	fly -t lite e -c ci/tasks/groot-tests.yml -p -i grootfs-git-repo=${PWD}

concourse-root-test:
	fly -t lite e -c ci/tasks/root-tests.yml -p -i grootfs-git-repo=${PWD}

concourse-test: concourse-groot-test concourse-root-test


###### Go tools ###############################################################

go-vet:
	GOOS=linux go vet $(go list ./... | grep -v vendor)

go-generate:
	GOOS=linux go generate $(go list ./... | grep -v vendor)

###### Docker #################################################################

image:
	docker build -t cfgarden/grootfs-ci .

push-image:
	docker push cfgarden/grootfs-ci

update-deps:
	./script/update-deps.sh


