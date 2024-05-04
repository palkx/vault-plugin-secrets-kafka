TEST?=./...
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)

default: build

build:
	go install

test:
	go test -v

testacc:
	TEST_KAFKA_USERNAME=root TEST_KAFKA_PASSWORD=rootpassword TEST_KAFKA_BOOTSTRAP_SERVERS=localhost:29092 TEST_KAFKA_USERNAME_PREFIX=svc VAULT_ACC=1 go test -timeout 120m

.PHONY: build test testacc
