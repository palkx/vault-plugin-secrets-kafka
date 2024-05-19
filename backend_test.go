package kafka

import (
	"context"
	"os"
	"testing"

	"github.com/IBM/sarama"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

const (
	envVarRunAccTests = "VAULT_ACC"

	envVarKafkaBootstrapServers = "TEST_KAFKA_BOOTSTRAP_SERVERS"
	envVarKafkaUsername         = "TEST_KAFKA_USERNAME"
	envVarKafkaPassword         = "TEST_KAFKA_PASSWORD"
	envVarKafkaCaBundle         = "TEST_KAFKA_CA_BUNDLE"
	envVarKafkaCertificate      = "TEST_KAFKA_CERTIFICATE"
	envVarKafkaCertificateKey   = "TEST_KAFKA_CERTIFICATE_KEY"
	envVarKafkaUsernamePrefix   = "TEST_KAFKA_USERNAME_PREFIX"
)

// getTestBackend will help you construct a test backend object.
// Update this function with your target backend.
func getTestBackend(tb testing.TB) (*kafkaBackend, logical.Storage) {
	tb.Helper()

	config := logical.TestBackendConfig()
	config.StorageView = new(logical.InmemStorage)
	config.Logger = hclog.NewNullLogger()
	config.System = logical.TestSystemView()

	b, err := Factory(context.Background(), config)
	if err != nil {
		tb.Fatal(err)
	}

	return b.(*kafkaBackend), config.StorageView
}

// runAcceptanceTests will separate unit tests from
// acceptance tests, which will make active requests
// to your target API.
var runAcceptanceTests = os.Getenv(envVarRunAccTests) == "1"

// testEnv creates an object to store and track testing environment
// resources
type testEnv struct {
	BootstrapServers      string
	Username              string
	Password              string
	CABundle              string
	Certificate           string
	CertificateKey        string
	UsernamePrefix        string
	ConfigScramSHAVersion string
	RoleScramSHAVersion   string
	RoleResourceACLs      string

	Backend logical.Backend
	Context context.Context
	Storage logical.Storage

	// Credentials tracks the generated tokens, to make sure we clean up
	Credentials []kafkaCredential
}

// AddConfig adds the configuration to the test backend.
// Make sure data includes all of the configuration
// attributes you need and the `config` path!
func (e *testEnv) AddConfig(t *testing.T) {
	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"bootstrap_servers": e.BootstrapServers,
			"username":          e.Username,
			"password":          e.Password,
			"ca_bundle":         e.CABundle,
			"certificate":       e.Certificate,
			"certificate_key":   e.CertificateKey,
			"scram_sha_version": e.ConfigScramSHAVersion,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

// AddConfig adds the configuration to the test backend.
// Make sure data includes all of the configuration
// attributes you need and the `config` path!
func (e *testEnv) DeleteConfig(t *testing.T) {
	req := &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "config",
		Storage:   e.Storage,
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

// AddUserTokenRole adds a role for the Kafka
// user credential.
func (e *testEnv) AddUserTokenRole(t *testing.T) {
	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "role/test-user-token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"username_prefix":   e.UsernamePrefix,
			"scram_sha_version": e.RoleScramSHAVersion,
			"resource_acls":     e.RoleResourceACLs,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

// ReadUserToken retrieves the user credential
// based on a Vault role.
func (e *testEnv) ReadUserCredential(t *testing.T) {
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "creds/test-user-token",
		Storage:   e.Storage,
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, err)
	require.NotNil(t, resp)

	require.NotEmpty(t, resp.Data["username"])
	require.NotEmpty(t, resp.Data["password"])

	b := e.Backend.(*kafkaBackend)
	client, err := b.getClient(e.Context, e.Storage)
	if err != nil {
		t.Fatal("fatal getting client")
	}

	users, err := client.DescribeUserScramCredentials([]string{resp.Data["username"].(string)})
	if err != nil {
		t.Fatalf("fatal describing user: %s", err)
	}

	for _, v := range users {
		require.Equal(t, resp.Data["username"], v.User)
		require.Equal(t, v.ErrorCode, sarama.ErrNoError)
		for _, ci := range v.CredentialInfos {
			switch e.RoleScramSHAVersion {
			case SCRAMSHA256:
				require.Equal(t, ci.Mechanism, sarama.SCRAM_MECHANISM_SHA_256)
			case SCRAMSHA512:
				require.Equal(t, ci.Mechanism, sarama.SCRAM_MECHANISM_SHA_512)
			}
			require.Equal(t, ci.Iterations, int32(8192))
		}
	}

	resourceACLs, err := parseResourceACLs(e.RoleResourceACLs, resp.Data["username"].(string))
	if err != nil {
		t.Fatalf("error parsing resource ACLs: %s", err)
	}
	ACLFilters, err := prepareACLFilters(resourceACLs)
	if err != nil {
		t.Fatalf("error preparing ACL filters: %s", err)
	}
	require.Len(t, ACLFilters, 4)

	for _, filter := range ACLFilters {
		acl, err := client.ListAcls(filter)
		if err != nil {
			t.Fatalf("error listing ACLs: %s", err)
		}
		require.Len(t, acl, 1)
		require.NotNil(t, acl)
		require.NotEmpty(t, acl[0].ResourceType)
		require.NotEmpty(t, acl[0].Resource)
		require.NotEmpty(t, acl[0].ResourcePatternType)
		require.NotEmpty(t, acl[0].Acls[0].Operation)
		require.NotEmpty(t, acl[0].Acls[0].Host)
		require.NotEmpty(t, acl[0].Acls[0].PermissionType)
		require.Equal(t, acl[0].Acls[0].Principal, resp.Data["username"].(string))
	}

	e.Credentials = append(e.Credentials, kafkaCredential{
		Username: resp.Data["username"].(string),
		Password: resp.Data["password"].(string),
	})
}

// CleanupUserTokens removes the tokens
// when the test completes.
func (e *testEnv) CleanupUserCredential(t *testing.T) {
	if len(e.Credentials) == 0 {
		t.Fatalf("expected 10 credentials, got: %d", len(e.Credentials))
	}

	b := e.Backend.(*kafkaBackend)
	client, err := b.getClient(e.Context, e.Storage)
	if err != nil {
		t.Fatal("fatal getting client")
	}

	for _, credential := range e.Credentials {
		err = deleteCredential(e.Context, client, credential.Username, e.RoleScramSHAVersion, e.RoleResourceACLs)
		if err != nil {
			t.Fatalf("error revoking Kafka credentials: %s", err)
		}

		resourceACLs, err := parseResourceACLs(e.RoleResourceACLs, credential.Username)
		if err != nil {
			t.Fatalf("error parsing resource ACLs: %s", err)
		}
		ACLFilters, err := prepareACLFilters(resourceACLs)
		if err != nil {
			t.Fatalf("error preparing ACL filters: %s", err)
		}
		require.Len(t, ACLFilters, 4)

		for _, filter := range ACLFilters {
			acl, err := client.ListAcls(filter)
			if err != nil {
				t.Fatalf("error listing ACLs: %s", err)
			}
			require.Len(t, acl, 0)
		}
	}

	e.Credentials = []kafkaCredential{}
}
