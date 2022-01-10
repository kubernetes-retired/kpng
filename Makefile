# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: all

all: help

# Auto Generate help from: https://gist.github.com/prwhite/8168133
# COLORS
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)
TARGET_MAX_CHAR_NUM=20

.PHONY: help

## Show help
help:
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-_0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			printf "  ${YELLOW}%-$(TARGET_MAX_CHAR_NUM)s${RESET} ${GREEN}%s${RESET}\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

# Dependecies
go_mod_tests_requirement:
	go install golang.org/x/lint/golint@latest

# Development mode - SKIP E2E
# Most of users use IPV4 and iptables as backend
# let's use it as default values

## Starts modd with the template available in-tree
modd:
	modd -f modd.conf.template

## Creates k8s cluster with IPV4 and IPTABLES (No E2E involved)
debug:
	./hack/test_e2e.sh -i ipv4 -b iptables -d
	
## Trigger unittests
test: go_mod_tests_requirement ## Execute unittests
	./hack/test_unit.sh

## E2E with IPV4 and IPTABLES
e2e-ipv4-iptables: go_mod_tests_requirement
	./hack/test_e2e.sh -i ipv4 -b iptables

## E2E with IPV6 and IPTABLES
e2e-ipv6-iptables: go_mod_tests_requirement
	./hack/test_e2e.sh -i ipv6 -b iptables

## E2E with DUAL and IPTABLES
e2e-dual-iptables: go_mod_tests_requirement
	./hack/test_e2e.sh -i dual -b iptables

## E2E with IPV4 and IPVS
e2e-ipv4-ipvs: go_mod_tests_requirement
	./hack/test_e2e.sh -i ipv4 -b ipvs

## E2E with IPV6 and IPVS
e2e-ipv6-ipvs: go_mod_tests_requirement
	./hack/test_e2e.sh -i ipv6 -b ipvs

## E2E with DUAL and IPVS
e2e-dual-ipvs: go_mod_tests_requirement
	./hack/test_e2e.sh -i dual -b ipvs

## E2E with IPV4 and NFT
e2e-ipv4-nft: go_mod_tests_requirement
	./hack/test_e2e.sh -i ipv4 -b nft

## E2E with IPV6 and NFT
e2e-ipv6-nft: go_mod_tests_requirement
	./hack/test_e2e.sh -i ipv6 -b nft

## E2E with DUAL and NFT
e2e-dual-nft: go_mod_tests_requirement
	./hack/test_e2e.sh -i dual -b nft

## Execute ALL E2E listed above
e2e: e2e-ipv4-iptables \
	e2e-ipv6-iptables \
	e2e-dual-iptables \
	e2e-ipv4-ipvs \
	e2e-ipv6-ipvs  \
	e2e-dual-ipvs \
	e2e-ipv4-nft \
	e2e-ipv6-nft \
	e2e-dual-nft
