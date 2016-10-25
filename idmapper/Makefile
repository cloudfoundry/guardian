.PHONY: all

all:
		GOOS=linux go build -o newuidmap ./cmd/newuidmap
		GOOS=linux go build -o newgidmap ./cmd/newgidmap

###### Help ###################################################################

help:
	@echo '    all ................................. builds the binaries'
	@echo '    test ................................ runs tests in concourse-lite'


###### Testing ################################################################

test:
		./hack/run-tests -g "-p" -r .

