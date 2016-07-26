.PHONY: all \
	test remote-test docker-test \
	image push-image

help:
	@echo '    all ................................. builds the grootfs cli'
	@echo '    concourse-test ...................... runs tests in concourse-lite'
	@echo '    docker-test ......................... runs tests in a docker container'
	@echo '    image ............................... builds a docker image'
	@echo '    push-image .......................... pushes image to docker-hub'
	@echo '    test ................................ runs tests locally'

###### Golang #################################################################

all:
	GOOS=linux go build .

###### Testing ################################################################

test:
	ginkgo -r -p -skipPackage integration .

concourse-test:
	fly -t lite e -c ci/tasks/grootfs.yml -i grootfs-git-repo=${PWD}

###### Docker #################################################################

image:
	docker build -t cfgarden/grootfs-ci .

push-image:
	docker push cfgarden/grootfs-ci
