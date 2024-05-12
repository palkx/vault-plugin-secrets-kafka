package kafka

import (
	"context"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"

	version "github.com/palkx/vault-plugin-secrets-kafka/version"
)

// Factory returns a new backend as logical.Backend
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := backend()
	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}
	return b, nil
}

// kafkaBackend defines an object that
// extends the Vault backend and stores the
// target API's client.
type kafkaBackend struct {
	*framework.Backend
	lock   sync.RWMutex
	client *kafkaClient
}

// backend defines the target API backend
// for Vault. It must include each path
// and the secrets it will store.
func backend() *kafkaBackend {
	b := kafkaBackend{}

	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(backendHelp),
		PathsSpecial: &logical.Paths{
			LocalStorage: []string{},
			SealWrapStorage: []string{
				"config",
				"role/*",
			},
		},
		Paths: framework.PathAppend(
			pathRole(&b),
			[]*framework.Path{
				pathConfig(&b),
				pathCredentials(&b),
			},
		),
		Secrets: []*framework.Secret{
			b.kafkaCredential(),
		},
		BackendType:    logical.TypeLogical,
		Invalidate:     b.invalidate,
		RunningVersion: "v" + version.Build(),
	}
	return &b
}

// reset clears any client configuration for a new
// backend to be configured
func (b *kafkaBackend) reset() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.client = nil
}

// invalidate clears an existing client configuration in
// the backend
func (b *kafkaBackend) invalidate(ctx context.Context, key string) {
	if key == "config" {
		b.reset()
	}
}

// getClient locks the backend as it configures and creates a
// a new client for the target API
func (b *kafkaBackend) getClient(ctx context.Context, s logical.Storage) (*kafkaClient, error) {
	b.lock.RLock()
	unlockFunc := b.lock.RUnlock
	clientAlive := true
	defer func() { unlockFunc() }()

	if b.client != nil {
		_, _, err := b.client.DescribeCluster()
		if err == nil {
			return b.client, nil
		}
		clientAlive = false
	}

	b.lock.RUnlock()

	if !clientAlive {
		b.client.Close()
		b.reset()
	}

	b.lock.Lock()
	unlockFunc = b.lock.Unlock

	// If the client was created during the lock switch, return it
	if b.client != nil {
		return b.client, nil
	}

	config, err := getConfig(ctx, s)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = new(kafkaConfig)
	}

	b.client, err = newClient(config)
	if err != nil {
		return nil, err
	}

	return b.client, nil
}

// backendHelp should contain help information for the backend
const backendHelp = `
The Kafka secrets backend dynamically generates user tokens.
After mounting this backend, credentials to manage Kafka user tokens
must be configured with the "config/" endpoints.
`
