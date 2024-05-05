package kafka

import (
	"context"
	"os"
	"testing"

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
	BootstrapServers string
	Username         string
	Password         string
	CABundle         string
	Certificate      string
	CertificateKey   string
	UsernamePrefix   string

	Backend logical.Backend
	Context context.Context
	Storage logical.Storage

	// Credentials tracks the generated tokens, to make sure we clean up
	Credentials []string
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
		},
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
			"username_prefix": e.UsernamePrefix,
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

	if t, ok := resp.Data["username"]; ok {
		e.Credentials = append(e.Credentials, t.(string))
	}

	require.NotEmpty(t, resp.Data["username"])
	require.NotEmpty(t, resp.Data["password"])
}

// CleanupUserTokens removes the tokens
// when the test completes.
func (e *testEnv) CleanupUserCredential(t *testing.T) {
	if len(e.Credentials) == 0 {
		t.Fatalf("expected 2 credentials, got: %d", len(e.Credentials))
	}

	for _, credential := range e.Credentials {
		b := e.Backend.(*kafkaBackend)
		client, err := b.getClient(e.Context, e.Storage)
		if err != nil {
			t.Fatal("fatal getting client")
		}

		err = deleteCredential(e.Context, client, credential)
		if err != nil {
			t.Fatalf("error revoking Kafka credentials: %s", err)
		}
	}
}
