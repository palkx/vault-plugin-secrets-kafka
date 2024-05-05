TEST?=./...
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)

default: build

upkafka:
	docker compose up kafka --wait --wait-timeout 240 -d

build:
	go install

test:
	go test -v 2>&1 ./...

testacc: upkafka
testacc:
	@TEST_KAFKA_BOOTSTRAP_SERVERS=127.0.0.1:29091 TEST_KAFKA_USERNAME=root TEST_KAFKA_PASSWORD=rootpassword TEST_KAFKA_CA_BUNDLE=$$(cat ./secrets/kafka/ca.pem|base64 -w0) TEST_KAFKA_CERTIFICATE=$$(cat ./secrets/kafka/kafka.pem|base64 -w0) TEST_KAFKA_CERTIFICATE_KEY=$$(cat ./secrets/kafka/kafka-key.pem|base64 -w0) TEST_KAFKA_USERNAME_PREFIX=svc VAULT_ACC=1 go test -v -timeout 10m 2>&1 ./...

cleanup:
	docker compose down -v
	rm -rf ./secrets/kafka

.PHONY: build test testkafka testacc upkafka cleanup
