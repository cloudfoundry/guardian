.PHONY: all test

all:
	GOOS=linux go build .

test:
	ginkgo -r -p .

remote-test:
	fly -t lite e -c ci/tasks/grootfs.yml -i grootfs-git-repo=${PWD}
