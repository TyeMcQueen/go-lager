SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
# Mac's gnu Make 3.81 does not support .ONESHELL:
# .ONESHELL:
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

# COLORS
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

## Show help
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
			odd = substr(" ", 0, 1-length(odd)); \
			printf "  ${YELLOW}%-15s ${GREEN}%s${RESET}%s\n", \
				helpCommand, helpMessage, odd; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST) \
		| sed -e '/[^ ]$$/s/   / . /g' -e 's/ $$//'

MOD := github.com/TyeMcQueen/go-lager/

cover: go.mod *.go */*.go
	go test -race -coverprofile cover ./...

cover.html: cover
	go tool cover -html cover -o cover.html

cover.txt: cover.html
	@grep '<option value=' cover.html | sed \
		-e 's:^.*<option value="[^"][^"]*">${MOD}::' -e 's:</option>.*::' \
		-e 's:^\([^ ][^ ]*\) [(]\(.*\)[)]:\2 \1:' | col-align - > cover.txt
	@echo ''

.PHONY: test
## Run unit tests and report statement coverage percentages
test: cover.txt
	@cat cover.txt

.PHONY: coverage
## View coverage details in your browser
coverage: cover.html
	open cover.html
