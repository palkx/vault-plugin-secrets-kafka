package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

// kafkaRoleEntry defines the data required
// for a Vault role to access and call the Kafka
// token endpoints
type kafkaRoleEntry struct {
	UsernamePrefix  string        `json:"username_prefix"`
	ScramSHAVersion string        `json:"scram_sha_version"`
	TTL             time.Duration `json:"ttl"`
	MaxTTL          time.Duration `json:"max_ttl"`
}

// toResponseData returns response data for a role
func (r *kafkaRoleEntry) toResponseData() map[string]interface{} {
	respData := map[string]interface{}{
		"username_prefix":   r.UsernamePrefix,
		"scram_sha_version": r.ScramSHAVersion,
		"ttl":               r.TTL.Seconds(),
		"max_ttl":           r.MaxTTL.Seconds(),
	}
	return respData
}

// pathRole extends the Vault API with a `/role`
// endpoint for the backend. You can choose whether
// or not certain attributes should be displayed,
// required, and named. You can also define different
// path patterns to list all roles.
func pathRole(b *kafkaBackend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: "role/" + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeLowerCaseString,
					Description: "Name of the role",
					Required:    true,
				},
				"username_prefix": {
					Type:        framework.TypeString,
					Description: "The username for the Kafka product API. Will match role name if not set.",
				},
				"scram_sha_version": {
					Type:        framework.TypeString,
					Description: fmt.Sprintf("Scram SHA Version to use for created credentials. Can be %s, %s. By default it will be set to the backends default value.", SCRAMSHA256, SCRAMSHA512),
				},
				"ttl": {
					Type:        framework.TypeDurationSecond,
					Description: "Default lease for generated credentials. If not set or set to 0, will use system default.",
				},
				"max_ttl": {
					Type:        framework.TypeDurationSecond,
					Description: "Maximum time for role. If not set or set to 0, will use system default.",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ReadOperation: &framework.PathOperation{
					Callback: b.pathRolesRead,
				},
				logical.CreateOperation: &framework.PathOperation{
					Callback: b.pathRolesWrite,
				},
				logical.UpdateOperation: &framework.PathOperation{
					Callback: b.pathRolesWrite,
				},
				logical.DeleteOperation: &framework.PathOperation{
					Callback: b.pathRolesDelete,
				},
			},
			ExistenceCheck:  b.pathRoleExistenceCheck,
			HelpSynopsis:    pathRoleHelpSynopsis,
			HelpDescription: pathRoleHelpDescription,
		},
		{
			Pattern: "role/?$",
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ListOperation: &framework.PathOperation{
					Callback: b.pathRolesList,
				},
			},
			HelpSynopsis:    pathRoleListHelpSynopsis,
			HelpDescription: pathRoleListHelpDescription,
		},
	}
}

// pathRoleExistenceCheck verifies if the configuration exists.
func (b *kafkaBackend) pathRoleExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	out, err := req.Storage.Get(ctx, req.Path)
	if err != nil {
		return false, fmt.Errorf("existence check failed: %w", err)
	}

	return out != nil, nil
}

func (b *kafkaBackend) getRole(ctx context.Context, s logical.Storage, name string) (*kafkaRoleEntry, error) {
	if name == "" {
		return nil, fmt.Errorf("missing role name")
	}

	entry, err := s.Get(ctx, "role/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var role kafkaRoleEntry

	if err := entry.DecodeJSON(&role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (b *kafkaBackend) pathRolesRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	entry, err := b.getRole(ctx, req.Storage, d.Get("name").(string))
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: entry.toResponseData(),
	}, nil
}

func setRole(ctx context.Context, s logical.Storage, name string, roleEntry *kafkaRoleEntry) error {
	entry, err := logical.StorageEntryJSON("role/"+name, roleEntry)
	if err != nil {
		return err
	}

	if entry == nil {
		return fmt.Errorf("failed to create storage entry for role")
	}

	if err := s.Put(ctx, entry); err != nil {
		return err
	}

	return nil
}

func (b *kafkaBackend) pathRolesWrite(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	name, ok := d.GetOk("name")
	if !ok {
		return logical.ErrorResponse("missing role name"), nil
	}

	roleEntry, err := b.getRole(ctx, req.Storage, name.(string))
	if err != nil {
		return nil, err
	}

	if roleEntry == nil {
		roleEntry = &kafkaRoleEntry{}
	}

	createOperation := (req.Operation == logical.CreateOperation)

	if usernamePrefix, ok := d.GetOk("username_prefix"); ok {
		roleEntry.UsernamePrefix = usernamePrefix.(string)
	} else if !ok && createOperation {
		roleEntry.UsernamePrefix = name.(string)
	}

	if scramSHAVersion, ok := d.GetOk("scram_sha_version"); ok {
		if scramSHAVersion.(string) != SCRAMSHA256 && scramSHAVersion.(string) != SCRAMSHA512 {
			return nil, fmt.Errorf("invalid scram_sha_version. Possible values are: %s, %s, but received: %s", SCRAMSHA256, SCRAMSHA512, scramSHAVersion.(string))
		}
		roleEntry.ScramSHAVersion = scramSHAVersion.(string)
	} else if !ok && createOperation {
		config, err := getConfig(ctx, req.Storage)
		if err != nil {
			return nil, err
		}
		roleEntry.ScramSHAVersion = config.ScramSHAVersion
	}

	if ttlRaw, ok := d.GetOk("ttl"); ok {
		roleEntry.TTL = time.Duration(ttlRaw.(int)) * time.Second
	} else if createOperation {
		roleEntry.TTL = time.Duration(d.Get("ttl").(int)) * time.Second
	}

	if maxTTLRaw, ok := d.GetOk("max_ttl"); ok {
		roleEntry.MaxTTL = time.Duration(maxTTLRaw.(int)) * time.Second
	} else if createOperation {
		roleEntry.MaxTTL = time.Duration(d.Get("max_ttl").(int)) * time.Second
	}

	if roleEntry.MaxTTL != 0 && roleEntry.TTL > roleEntry.MaxTTL {
		return logical.ErrorResponse("ttl cannot be greater than max_ttl"), nil
	}

	if err := setRole(ctx, req.Storage, name.(string), roleEntry); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *kafkaBackend) pathRolesDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, "role/"+d.Get("name").(string))
	if err != nil {
		return nil, fmt.Errorf("error deleting kafka role: %w", err)
	}

	return nil, nil
}

func (b *kafkaBackend) pathRolesList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, "role/")
	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

const (
	pathRoleHelpSynopsis    = `Manages the Vault role for generating Kafka tokens.`
	pathRoleHelpDescription = `
This path allows you to read and write roles used to generate Kafka tokens.
You can configure a role to manage a user's token by setting the username field.
`

	pathRoleListHelpSynopsis    = `List the existing roles in Kafka backend`
	pathRoleListHelpDescription = `Roles will be listed by the role name.`
)
