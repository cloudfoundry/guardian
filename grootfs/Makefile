.PHONY: all \
	test remote-test docker-test \
	image push-image

all:
	GOOS=linux go build .

###### Help ###################################################################

help:
	@echo '    all ................................. builds the grootfs cli'
	@echo '    concourse-test ...................... runs tests in concourse-lite'
	@echo '    image ............................... builds a docker image'
	@echo '    push-image .......................... pushes image to docker-hub'
	@echo '    test ................................ runs tests locally'

###### Testing ################################################################

test:
	ginkgo -r -p -skipPackage integration .

concourse-test:
	fly -t lite e -c ci/tasks/grootfs.yml -p -i grootfs-git-repo=${PWD}

###### Docker #################################################################

image:
	docker build -t cfgarden/grootfs-ci .

push-image:
	docker push cfgarden/grootfs-ci
