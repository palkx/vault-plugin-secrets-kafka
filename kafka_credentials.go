package kafka

import (
	"context"
	"encoding/json"
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
	ResourceACLs    string `json:"resource_acls"`
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
			"resource_acls": {
				Type:        framework.TypeString,
				Description: "ACLs created for the user",
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

func parseResourceACLs(acls string, principalName string) ([]*sarama.ResourceAcls, error) {
	var resourceACLs []*sarama.ResourceAcls
	err := json.Unmarshal([]byte(acls), &resourceACLs)
	if err != nil {
		return nil, err
	}
	for _, ra := range resourceACLs {
		if ra.ResourceType == sarama.AclResourceUnknown {
			return nil, errors.New("received unknown resource type")
		}
		if ra.ResourceName == "" {
			return nil, errors.New("received empty resource name")
		}
		if ra.ResourcePatternType == sarama.AclPatternUnknown {
			return nil, errors.New("received unknown resource pattern type")
		}
		if ra.Acls == nil {
			return nil, errors.New("received empty resource acls")
		}
		for _, a := range ra.Acls {
			if a.Host == "" {
				return nil, errors.New("received empty host in resource ACL")
			}
			if a.Operation == sarama.AclOperationUnknown {
				return nil, errors.New("received unknown operation in resource ACL")
			}
			if a.PermissionType == sarama.AclPermissionUnknown {
				return nil, errors.New("received unknown permission type in resource ACL")
			}
			a.Principal = principalName
		}
	}
	return resourceACLs, nil
}

func prepareACLFilters(resourceACLs []*sarama.ResourceAcls) ([]sarama.AclFilter, error) {
	var aclFilters []sarama.AclFilter

	for _, ra := range resourceACLs {
		for _, a := range ra.Acls {
			aclFilters = append(aclFilters, sarama.AclFilter{
				ResourceType:              ra.ResourceType,
				ResourceName:              &ra.ResourceName,
				ResourcePatternTypeFilter: ra.ResourcePatternType,
				Principal:                 &a.Principal,
				Host:                      &a.Host,
				Operation:                 a.Operation,
				PermissionType:            a.PermissionType,
			})
		}
	}

	return aclFilters, nil
}

func deleteCredential(ctx context.Context, c *kafkaClient, username string, scramSHAVersion string, resourceACLs string) error {
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

	if resourceACLs != "" {
		// Prepare ACLs with generated username
		userResourceACLs, err := parseResourceACLs(resourceACLs, username)
		if err != nil {
			return err
		}

		aclFilters, err := prepareACLFilters(userResourceACLs)
		if err != nil {
			return err
		}

		// Delete all ACLs
		for _, filter := range aclFilters {
			_, err = c.DeleteACL(filter, false)
			if err != nil {
				return err
			}
		}
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

	resourceACLs := ""
	resourceACLsRaw, ok := req.Secret.InternalData["resource_acls"]
	if ok {
		resourceACLs, _ = resourceACLsRaw.(string)
	}

	if err := deleteCredential(ctx, client, username, scramSHAVersion, resourceACLs); err != nil {
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

	if roleEntry.ResourceACLs != "" {
		// Prepare ACLs with generated username
		resourceACLs, err := parseResourceACLs(roleEntry.ResourceACLs, username)
		if err != nil {
			return nil, err
		}

		// Create all ACLs
		err = c.CreateACLs(resourceACLs)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, fmt.Errorf("error creating Kafka credentials: %w", err)
	}

	return &kafkaCredential{
		Username: username,
		Password: password,
		// We're saving resource acls from the role instead of credential ACLs
		// because we won't be able to unmarhal (to save the) and marshal credential ACLs once again
		ResourceACLs:    roleEntry.ResourceACLs,
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
