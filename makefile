#!/bin/sh

MIN_COV = 100.0

bin/: ; mkdir -p $@

bin/golangci-lint: | bin/
	GOBIN="$(realpath $(dir $@))" go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.3

.PHONY: lint
lint: | bin/golangci-lint ## Run code linters
	@PATH="$(realpath bin):$$PATH" golangci-lint run


.PHONY: test
test:
	go test ./... -v -cover -coverpkg=./... -coverprofile=cov.out

.PHONY: cov
cov:
	@go tool cover -func cov.out | grep total | xargs -I {} | awk '{print substr($$3, 1, length($$3)-1)}'

.PHONY: check-cov
check-cov:
	@$(eval COV:=$(shell make cov))
	@echo $(shell echo ${COV}\>=${MIN_COV} | bc) | grep 1  > /dev/null || false | (echo 'current coverage ${COV} lower then ${MIN_COV}'; exit 1)
