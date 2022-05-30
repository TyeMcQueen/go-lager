# COLORS
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

# Variables
GOPRIVATE?="github.com/Unity-Technologies/*"
GO=GOPRIVATE=${GOPRIVATE} go
GOPROXY?=https://athens.prd.cds.internal.unity3d.com/

.PHONY: help
help:
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_\/0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			printf "  ${YELLOW}%-10s ${GREEN}%s${RESET}\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.PHONY: deps
## Install and verify dependencies.
deps:
	go mod tidy
	go mod verify


.PHONY: format
## Run format on entire project
format:
	go fmt ./...

.PHONY: test
## Run tests
test:
	go test -v -race -timeout 30s ./...
