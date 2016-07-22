.PHONY: all \
	test remote-test docker-test \
	image push-image

###### Golang #################################################################

all:
	GOOS=linux go build .

###### Testing ################################################################

test:
	ginkgo -r -p .

concourse-test:
	fly -t lite e -c ci/tasks/grootfs.yml -i grootfs-git-repo=${PWD}

docker-test:
	docker run --rm --name grootfs-test \
		-v ${PWD}:/go/src/code.cloudfoundry.org/grootfs \
		cfgarden/grootfs-ci \
		make test

###### Docker #################################################################

image:
	docker build -t cfgarden/grootfs-ci .

push-image:
	docker push cfgarden/grootfs-ci
