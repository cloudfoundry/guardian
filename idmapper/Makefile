.PHONY: all

all:
		GOOS=linux go build -mod vendor -o newuidmap ./cmd/newuidmap
		GOOS=linux go build -mod vendor -o newgidmap ./cmd/newgidmap
		GOOS=linux go build -mod vendor -o maximus ./cmd/maximus

###### Help ###################################################################

help:
	@echo '    all ................................. builds the binaries'
	@echo '    test ................................ runs tests in concourse-lite'


###### Testing ################################################################

test:
		./hack/run-tests -g "-p" -r .

