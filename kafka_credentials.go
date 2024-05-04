package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	kafkaCredentialType = "kafka_credential"
)

// kafkaCredentials defines a secret for the Kafka credentials
type kafkaCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// kafkaCredentials defines a secret to store for a given role
// and how it should be revoked or renewed.
func (b *kafkaBackend) kafkaCredential() *framework.Secret {
	return &framework.Secret{
		Type: kafkaCredentialType,
		Fields: map[string]*framework.FieldSchema{
			"username": {
				Type:        framework.TypeString,
				Description: "Kafka User Username",
			},
			"password": {
				Type:        framework.TypeString,
				Description: "Kafka User Password",
			},
		},
		Revoke: b.credentialRevoke,
		Renew:  b.credentialRenew,
	}
}

func deleteCredential(ctx context.Context, c *kafkaClient, username string) error {
	if username == "" {
		return fmt.Errorf("recieved empty username for credential deletion")
	}

	_, err := c.DeleteUserScramCredentials([]sarama.AlterUserScramCredentialsDelete{
		{
			Name:      username,
			Mechanism: sarama.SCRAM_MECHANISM_SHA_512,
		},
	})
	if err != nil {
		return fmt.Errorf("error creating Kafka credentials: %w", err)
	}

	return nil
}

// credentialsRevoke removes the credentials from the Vault storage API and calls the client to revoke the credentials
func (b *kafkaBackend) credentialRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}

	username := ""
	usernameRaw, ok := req.Secret.InternalData["username"]
	if ok {
		username, ok = usernameRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for username in secret internal data")
		}
	}

	if err := deleteCredential(ctx, client, username); err != nil {
		return nil, fmt.Errorf("error revoking user credentials: %w", err)
	}

	return nil, nil
}

func createCredential(ctx context.Context, c *kafkaClient, usernamePrefix string) (*kafkaCredential, error) {
	username := usernamePrefix + "-credential-" + uuid.New().String()
	password := uuid.New().String()
	salt := uuid.New().String()

	_, err := c.UpsertUserScramCredentials([]sarama.AlterUserScramCredentialsUpsert{
		{
			Name:       username,
			Mechanism:  sarama.SCRAM_MECHANISM_SHA_512,
			Iterations: 8192,
			Salt:       []byte(salt),
			Password:   []byte(password),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka credentials: %w", err)
	}

	return &kafkaCredential{
		Username: username,
		Password: password,
	}, nil
}

// credentialsRenew calls the client to create a new credentials and stores it in the Vault storage API
func (b *kafkaBackend) credentialRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleRaw, ok := req.Secret.InternalData["role"]
	if !ok {
		return nil, fmt.Errorf("secret is missing role internal data")
	}

	role := roleRaw.(string)
	roleEntry, err := b.getRole(ctx, req.Storage, role)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %w", err)
	}

	if roleEntry == nil {
		return nil, errors.New("error retrieving role: role is nil")
	}

	resp := &logical.Response{Secret: req.Secret}

	if roleEntry.TTL > 0 {
		resp.Secret.TTL = roleEntry.TTL
	}
	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
	}

	return resp, nil
}
