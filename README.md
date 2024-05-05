# Vault Secrets Kafka Plugin

## Description

This plugin is still in the development stage.

Main goal is to achieve dynamic user generation/revocation in the Kafka clusters
with SASL_SCRAM auth mechanism.

Currently it's working with SCRAM_SHA512 mechanism.

To perform local testing run the following commands:

1. `docker compose up -d`
2. `docker compose exec vault ash`
3. `vault secrets enable -path=kafka kafka`
4. `vault write kafka/config username=root password=rootpassword bootstrap_servers=kafka:29092 ca_bundle=$(cat /secrets/ca.pem|base64 -w0) certificate=$(cat /secrets/kafka.pem|base64 -w0) certificate_key=$(cat /secrets/kafka-key.pem|base64 -w0)`
5. `vault write -force kafka/role/svc`
6. `vault read kafka/creds/svc` (should successfully create a user in the kafka
   cluster)

## Roadmap

- [x] Initialize plugin skeleton
- [x] Write first working prototype
- [x] Remove dependency on confluent-kafka go plugin (it's using C dependencies
      and because of this we can't perform statically linked builds)
- [x] Configure basic CI/CD flow
- [x] Revisit kafka client lifecycle
- [x] Improve credential revocation during service disruption events
- [ ] Select SCRAM-SHA version on the plugin config level
- [x] Specify TLS certificates and CA on the plugin config level
- [ ] Revisit tests
- [ ] Revisit acceptance tests
