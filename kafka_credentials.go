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
	Username        string `json:"username"`
	Password        string `json:"password"`
	ScramSHAVersion string `json:"scram_sha_version"`
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
			"scram_sha_version": {
				Type:        framework.TypeString,
				Description: "Kafka Credential SCRAM SHA Version",
			},
		},
		Revoke: b.credentialRevoke,
		Renew:  b.credentialRenew,
	}
}

func deleteCredential(ctx context.Context, c *kafkaClient, username string, scramSHAVersion string) error {
	if username == "" {
		return fmt.Errorf("recieved empty username for credential deletion")
	}

	if scramSHAVersion != SCRAMSHA512 && scramSHAVersion != SCRAMSHA256 {
		return fmt.Errorf("invalid scram_sha_version for credential deletion. Can be %s, %s but recieved %s", SCRAMSHA256, SCRAMSHA512, scramSHAVersion)
	}

	mechanism := sarama.SCRAM_MECHANISM_SHA_512

	if scramSHAVersion == SCRAMSHA256 {
		mechanism = sarama.SCRAM_MECHANISM_SHA_256
	}

	_, err := c.DeleteUserScramCredentials([]sarama.AlterUserScramCredentialsDelete{
		{
			Name:      username,
			Mechanism: mechanism,
		},
	})
	if err != nil {
		return fmt.Errorf("error revoking Kafka credentials: %w", err)
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

	scramSHAVersion := ""
	scramSHAVersionRaw, ok := req.Secret.InternalData["scram_sha_version"]
	if ok {
		scramSHAVersion, ok = scramSHAVersionRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for scram_sha_version in secret internal data")
		}
	}

	if err := deleteCredential(ctx, client, username, scramSHAVersion); err != nil {
		return nil, err
	}

	return nil, nil
}

func createCredential(ctx context.Context, c *kafkaClient, roleEntry *kafkaRoleEntry, displayName string) (*kafkaCredential, error) {
	name := displayName

	if displayName == "" {
		name = "anonymous"
	}

	username := roleEntry.UsernamePrefix + "-" + name + "-" + uuid.New().String()
	password := uuid.New().String()
	salt := uuid.New().String()
	mechanism := sarama.SCRAM_MECHANISM_SHA_512

	if roleEntry.ScramSHAVersion == SCRAMSHA256 {
		mechanism = sarama.SCRAM_MECHANISM_SHA_256
	}

	_, err := c.UpsertUserScramCredentials([]sarama.AlterUserScramCredentialsUpsert{
		{
			Name:       username,
			Mechanism:  mechanism,
			Iterations: 8192,
			Salt:       []byte(salt),
			Password:   []byte(password),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka credentials: %w", err)
	}

	return &kafkaCredential{
		Username:        username,
		Password:        password,
		ScramSHAVersion: roleEntry.ScramSHAVersion,
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
