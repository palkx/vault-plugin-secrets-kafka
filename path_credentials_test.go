package kafka

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/logical"
)

// newAcceptanceTestEnv creates a test environment for credentials
func newAcceptanceTestEnv() (*testEnv, error) {
	ctx := context.Background()

	maxLease, _ := time.ParseDuration("60s")
	defaultLease, _ := time.ParseDuration("30s")
	conf := &logical.BackendConfig{
		System: &logical.StaticSystemView{
			DefaultLeaseTTLVal: defaultLease,
			MaxLeaseTTLVal:     maxLease,
		},
		Logger: logging.NewVaultLogger(log.Debug),
	}
	b, err := Factory(ctx, conf)
	if err != nil {
		return nil, err
	}
	return &testEnv{
		BootstrapServers:      os.Getenv(envVarKafkaBootstrapServers),
		Username:              os.Getenv(envVarKafkaUsername),
		Password:              os.Getenv(envVarKafkaPassword),
		CABundle:              os.Getenv(envVarKafkaCaBundle),
		Certificate:           os.Getenv(envVarKafkaCertificate),
		CertificateKey:        os.Getenv(envVarKafkaCertificateKey),
		UsernamePrefix:        os.Getenv(envVarKafkaUsernamePrefix),
		ConfigScramSHAVersion: SCRAMSHA512,
		RoleScramSHAVersion:   SCRAMSHA512,
		RoleResourceACLs:      `[{"ResourceType":"topic","ResourceName":"test-topic","ResourcePatternType":"literal","Acls":[{"Host":"*","Operation":"read","PermissionType":"allow"},{"Host":"*","Operation":"write","PermissionType":"allow"},{"Host":"*","Operation":"describeConfigs","PermissionType":"allow"}]},{"ResourceType":"topic","ResourceName":"test-topic2","ResourcePatternType":"prefixed","Acls":[{"Host":"*","Operation":"all","PermissionType":"allow"}]}]`,
		Backend:               b,
		Context:               ctx,
		Storage:               &logical.InmemStorage{},
	}, nil
}

// TestAcceptanceUserToken tests a series of steps to make
// sure the role and token creation work correctly.
func TestAcceptanceUserToken(t *testing.T) {
	if !runAcceptanceTests {
		t.SkipNow()
	}

	acceptanceTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run(fmt.Sprintf("add config with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.AddConfig)
	t.Run(fmt.Sprintf("add user credential role with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.AddUserTokenRole)
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("read user credential with scram %s %d/10", acceptanceTestEnv.RoleScramSHAVersion, i+1), acceptanceTestEnv.ReadUserCredential)
	}
	t.Run(fmt.Sprintf("cleanup user credentials with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.CleanupUserCredential)
	t.Run(fmt.Sprintf("delete config with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.DeleteConfig)

	acceptanceTestEnv.RoleScramSHAVersion = SCRAMSHA256
	t.Run(fmt.Sprintf("add config with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.AddConfig)
	t.Run(fmt.Sprintf("add user credential role with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.AddUserTokenRole)
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("read user credential with scram %s %d/10", acceptanceTestEnv.RoleScramSHAVersion, i+1), acceptanceTestEnv.ReadUserCredential)
	}
	t.Run(fmt.Sprintf("cleanup user credentials with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.CleanupUserCredential)
	t.Run(fmt.Sprintf("delete config with scram %s", acceptanceTestEnv.RoleScramSHAVersion), acceptanceTestEnv.DeleteConfig)
}
