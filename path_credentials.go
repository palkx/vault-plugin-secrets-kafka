package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

// pathCredentials extends the Vault API with a `/creds`
// endpoint for a role. You can choose whether
// or not certain attributes should be displayed,
// required, and named.
func pathCredentials(b *kafkaBackend) *framework.Path {
	return &framework.Path{
		Pattern: "creds/" + framework.GenericNameRegex("name"),
		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeLowerCaseString,
				Description: "Name of the role",
				Required:    true,
			},
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   b.pathCredentialsRead,
			logical.CreateOperation: b.pathCredentialsRead,
		},
		ExistenceCheck:  b.pathCredentialsExistenceCheck,
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

// pathCredentialsExistenceCheck verifies if the configuration exists.
func (b *kafkaBackend) pathCredentialsExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	out, err := req.Storage.Get(ctx, req.Path)
	if err != nil {
		return false, fmt.Errorf("existence check failed: %w", err)
	}

	return out != nil, nil
}

func (b *kafkaBackend) createCredentials(ctx context.Context, s logical.Storage, roleEntry *kafkaRoleEntry) (*kafkaCredential, error) {
	client, err := b.getClient(ctx, s)
	if err != nil {
		return nil, err
	}

	var credential *kafkaCredential

	credential, err = createCredential(ctx, client, roleEntry.UsernamePrefix)
	if err != nil {
		return nil, fmt.Errorf("error creating Kafka credential: %w", err)
	}

	if credential == nil {
		return nil, errors.New("error creating Kafka credential")
	}

	return credential, nil
}

func (b *kafkaBackend) createUserCreds(ctx context.Context, req *logical.Request, role *kafkaRoleEntry) (*logical.Response, error) {
	credential, err := b.createCredentials(ctx, req.Storage, role)
	if err != nil {
		return nil, err
	}

	resp := b.Secret(kafkaCredentialType).Response(map[string]interface{}{
		"password": credential.Password,
		"username": credential.Username,
	}, map[string]interface{}{
		"password": credential.Password,
		"username": credential.Username,
	})

	if role.TTL > 0 {
		resp.Secret.TTL = role.TTL
	}

	if role.MaxTTL > 0 {
		resp.Secret.MaxTTL = role.MaxTTL
	}

	return resp, nil
}

func (b *kafkaBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleName := d.Get("name").(string)

	roleEntry, err := b.getRole(ctx, req.Storage, roleName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %w", err)
	}

	if roleEntry == nil {
		return nil, errors.New("error retrieving role: role is nil")
	}

	return b.createUserCreds(ctx, req, roleEntry)
}

const pathCredentialsHelpSyn = `
Generate a Kafka API token from a specific Vault role.
`

const pathCredentialsHelpDesc = `
This path generates a Kafka API user tokens
based on a particular role. A role can only represent a user token,
since Kafka doesn't have other types of tokens.
`
