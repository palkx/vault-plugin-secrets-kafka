# Vault Secrets Kafka Plugin

## Description

This is a vault secrets plugin for kafka.

This plugin integrates with kafka clusters with SASL_SSL SCRAM-SHA-256 and
SASL_SSL SCRAM-SHA-512 authentication mechanisms.

## Installation

1. Download the plugin from the releases page:
   `curl -LO https://github.com/palkx/vault-plugin-secrets-kafka/releases/download/v0.0.4/vault-plugin-secrets-kafka_0.0.4_linux_amd64.zip`
2. Unzip it: `unzip vault-plugin-secrets-kafka_0.0.4_linux_amd64.zip`
3. Copy kafka plugin to the vault plugins directory. For example if you set
   `plugin_directory = "/vault/plugins"` in you vault server config:
   `cp kafka /vault/plugins/`
4. Start vault, unseal it and register the plugin:
   `vault plugin register -sha256="$(sha256sum /vault/plugins/kafka | awk '{print $1}')" secret kafka`
5. Create new backend: `vault secrets enable -path=kafka kafka`
6. Backend configuration supports the following options:
   - `username`: SASL username. This user will be used to create
     credentials/ACLs in your kafka cluster.
   - `password`: SASL password.
   - `bootstrap_servers`: comma separated list of kafka brokers.
   - `ca_bundle` (optional): Base64 encoded CA certificates. If present they're
     will be used to verify the server certificate.
   - `certificate` (optional): Base64 encoded client certificate. If specified
     this certificate will be provided to authenticate in Kafka cluster. When
     provided the `certificate_key` must be provided as well.
   - `certificate_key` (optional): Base64 encoded client certificate key. If
     specified this key will be used to authenticate in Kafka cluster.
   - `scram_sha_version` (optional): SCRAM-SHA version. The default is `512`.
     Valid values are `256` and `512`.
7. Example backend configuration:
   `vault write kafka/config username=root password=rootpassword bootstrap_servers=kafka:29092 ca_bundle=$(cat /secrets/ca.pem|base64 -w0) certificate=$(cat /secrets/kafka.pem|base64 -w0) certificate_key=$(cat /secrets/kafka-key.pem|base64 -w0) scram_sha_version=512`
8. Role configuration supports the following options:
   - `username_prefix` (optional): prefix, which will be for credentials. If not
     specified defaults to role name.
   - `scram_sha_version` (optional): SCRAM-SHA version, which will be for
     credentials. The default is `512`. Valid values are `256` and `512`.
   - `resource_acls` (optional): JSON encoded array of ACL permissions to be
     created for the role credentials. This set of ACLs is created for each
     credential created and will be deleted on lease expire. By default it's
     empty. [JSON structure described below.](#acl-json-structure)
   - `ttl` (optional): The TTL of the role credentials. Specified in seconds.
   - `max_ttl` (optional): The Maximum TTL of the role credentials. Specified in
     seconds.
9. Example role configuration:
   `vault write kafka/role/svc resource_acls='[{"ResourceType":"topic","ResourceName":"test-topic","ResourcePatternType":"literal","Acls":[{"Host":"*","Operation":"read","PermissionType":"allow"},{"Host":"*","Operation":"write","PermissionType":"allow"},{"Host":"*","Operation":"describeConfigs","PermissionType":"allow"}]}]'`
10. Read role credentials: `vault read kafka/creds/svc`. Example output:

    ```sh
    vault read kafka/creds/svc
    Key                Value
    ---                -----
    lease_id           kafka/creds/svc/7RC40tu6fwd5ZzUlJsj63A0e
    lease_duration     768h
    lease_renewable    true
    password           c70f0d84-ce17-4644-8e58-f842636b66c3
    username           svc-token-ae6e9d30-bdd1-4d3a-b609-c074600dede0
    ```

## ACL JSON Structure

Example JSON structure:

```json
[
  {
    "ResourceType": "Topic",
    "ResourceName": "test-topic",
    "ResourcePatternType": "Literal",
    "Acls": [
      {
        "Host": "*",
        "Operation": "Read",
        "PermissionType": "Allow"
      },
      {
        "Host": "*",
        "Operation": "Write",
        "PermissionType": "Allow"
      },
      {
        "Host": "*",
        "Operation": "DescribeConfigs",
        "PermissionType": "Allow"
      }
    ]
  },
  {
    "ResourceType": "Topic",
    "ResourceName": "test-topic2",
    "ResourcePatternType": "Prefixed",
    "Acls": [
      {
        "Host": "*",
        "Operation": "All",
        "PermissionType": "Allow"
      }
    ]
  }
]
```

For all available options and values check
[kafka ACLs documentation](https://docs.confluent.io/platform/current/kafka/authorization.html#principal)

To compact this json you can use `jq`. For example: `jq -rcM '.' acls.json`

## Local development

Currently we're using go 1.23.x.

To perform local build and test run the following commands:

1. Compile the plugin, launch kafka with SASL_SSL SCRAM-SHA-512 and run the
   vault server: `docker compose up -d`
2. Exec into the vault container: `docker compose exec vault ash`
3. Enable our plugin in the vault: `vault secrets enable -path=kafka kafka`
4. Configure the plugin:
   `vault write kafka/config username=root password=rootpassword bootstrap_servers=kafka:29092 ca_bundle=$(cat /secrets/ca.pem|base64 -w0) certificate=$(cat /secrets/kafka.pem|base64 -w0) certificate_key=$(cat /secrets/kafka-key.pem|base64 -w0)`
5. Create a role:
   `vault write kafka/role/svc resource_acls='[{"ResourceType":"topic","ResourceName":"test-topic","ResourcePatternType":"literal","Acls":[{"Host":"*","Operation":"read","PermissionType":"allow"},{"Host":"*","Operation":"write","PermissionType":"allow"},{"Host":"*","Operation":"describeConfigs","PermissionType":"allow"}]},{"ResourceType":"topic","ResourceName":"test-topic2","ResourcePatternType":"prefixed","Acls":[{"Host":"*","Operation":"all","PermissionType":"allow"}]}]'`
6. Generate credentials: `vault read kafka/creds/svc` (should successfully
   create a user in the kafka cluster)

## Roadmap

- [x] Initialize plugin skeleton
- [x] Write first working prototype
- [x] Remove dependency on confluent-kafka go plugin (it's using C dependencies
      and because of this we can't perform statically linked builds)
- [x] Configure basic CI/CD flow
- [x] Revisit kafka client lifecycle
- [x] Improve credential revocation during service disruption events
- [x] Select SCRAM-SHA version on the plugin config/role level
- [x] Specify TLS certificates and CA on the plugin config level
- [x] Revisit tests
- [x] Revisit acceptance tests
- [x] Add ability to manage ACLs
