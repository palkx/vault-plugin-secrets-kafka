package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	kafkaTokenType = "kafka_token"
)

// kafkaToken defines a secret for the Kafka token
type kafkaToken struct {
	Username string `json:"username"`
	TokenID  string `json:"token_id"`
	Token    string `json:"token"`
}

// kafkaToken defines a secret to store for a given role
// and how it should be revoked or renewed.
func (b *kafkaBackend) kafkaToken() *framework.Secret {
	return &framework.Secret{
		Type: kafkaTokenType,
		Fields: map[string]*framework.FieldSchema{
			"token": {
				Type:        framework.TypeString,
				Description: "Kafka Token",
			},
		},
		Revoke: b.tokenRevoke,
		Renew:  b.tokenRenew,
	}
}

func deleteToken(ctx context.Context, c *kafkaClient, token string) error {
	// TODO: implement this
	// c.Client.Token = token
	// err := c.SignOut()
	// if err != nil {
	// 	return err
	// }

	return nil
}

// tokenRevoke removes the token from the Vault storage API and calls the client to revoke the token
func (b *kafkaBackend) tokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}

	token := ""
	tokenRaw, ok := req.Secret.InternalData["token"]
	if ok {
		token, ok = tokenRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for token in secret internal data")
		}
	}

	if err := deleteToken(ctx, client, token); err != nil {
		return nil, fmt.Errorf("error revoking user token: %w", err)
	}

	return nil, nil
}

func createToken(ctx context.Context, c *kafkaClient, username string) (*kafkaToken, error) {
	// TODO: implement this
	// response, err := c.SignIn()
	// if err != nil {
	// 	return nil, fmt.Errorf("error creating Kafka token: %w", err)
	// }

	tokenID := uuid.New().String()
	mockToken := uuid.New().String()

	return &kafkaToken{
		Username: username,
		TokenID:  tokenID,
		Token:    mockToken,
	}, nil
}

// tokenRenew calls the client to create a new token and stores it in the Vault storage API
func (b *kafkaBackend) tokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
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
