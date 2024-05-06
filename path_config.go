package kafka

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	configStoragePath = "config"
	SCRAMSHA256       = "256"
	SCRAMSHA512       = "512"
)

// kafkaConfig includes the minimum configuration
// required to instantiate a new kafkaConfig client.
type kafkaConfig struct {
	BootstrapServers string `json:"bootstrap_servers"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	CABundle         string `json:"ca_bundle"`
	Certificate      string `json:"certificate"`
	CertificateKey   string `json:"certificate_key"`
	ScramSHAVersion  string `json:"scram_sha_version"`
}

// pathConfig extends the Vault API with a `/config`
// endpoint for the backend. You can choose whether
// or not certain attributes should be displayed,
// required, and named. For example, password
// is marked as sensitive and will not be output
// when you read the configuration.
func pathConfig(b *kafkaBackend) *framework.Path {
	return &framework.Path{
		Pattern: "config",
		Fields: map[string]*framework.FieldSchema{
			"bootstrap_servers": {
				Type:        framework.TypeString,
				Description: "The Bootstrap servers for Kafka",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Bootstrap Servers",
					Sensitive: false,
				},
			},
			"username": {
				Type:        framework.TypeString,
				Description: "The username to access Kafka",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Username",
					Sensitive: false,
				},
			},
			"password": {
				Type:        framework.TypeString,
				Description: "The password to access Kafka",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Password",
					Sensitive: true,
				},
			},
			"ca_bundle": {
				Type:        framework.TypeString,
				Description: "CA certificates to trust for Kafka. Must be provided as Base64 encoded PEM.",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "CA Bundle",
					Sensitive: false,
				},
			},
			"certificate": {
				Type:        framework.TypeString,
				Description: "Client certificate for Kafka. Must be provided as Base64 encoded PEM.",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Certificate",
					Sensitive: false,
				},
			},
			"certificate_key": {
				Type:        framework.TypeString,
				Description: "Client certificate key for Kafka. Must be provided as Base64 encoded PEM.",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Certificate Key",
					Sensitive: true,
				},
			},
			"scram_sha_version": {
				Type:        framework.TypeString,
				Description: fmt.Sprintf("SCRAM SHA Version to use. Possible values are: %s, %s. Defaults to %s.", SCRAMSHA256, SCRAMSHA512, SCRAMSHA512),
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "SCRAM SHA Version",
					Sensitive: false,
				},
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
			},
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
			},
		},
		ExistenceCheck:  b.pathConfigExistenceCheck,
		HelpSynopsis:    pathConfigHelpSynopsis,
		HelpDescription: pathConfigHelpDescription,
	}
}

func (b *kafkaBackend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"bootstrap_servers": config.BootstrapServers,
			"username":          config.Username,
			"ca_bundle":         config.CABundle,
			"certificate":       config.Certificate,
			"scram_sha_version": config.ScramSHAVersion,
		},
	}, nil
}

func (b *kafkaBackend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	createOperation := (req.Operation == logical.CreateOperation)

	if config == nil {
		if !createOperation {
			return nil, errors.New("config not found during update operation")
		}
		config = new(kafkaConfig)
	}

	if bootstrap_servers, ok := data.GetOk("bootstrap_servers"); ok {
		config.BootstrapServers = bootstrap_servers.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing bootstrap_servers in configuration")
	}

	if username, ok := data.GetOk("username"); ok {
		config.Username = username.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing username in configuration")
	}

	if password, ok := data.GetOk("password"); ok {
		config.Password = password.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing password in configuration")
	}

	if scramSHAVersion, ok := data.GetOk("scram_sha_version"); ok {
		if scramSHAVersion.(string) != SCRAMSHA256 && scramSHAVersion.(string) != SCRAMSHA512 {
			return nil, fmt.Errorf("invalid scram_sha_version. Possible values are: %s, %s, but received: %s", SCRAMSHA256, SCRAMSHA512, scramSHAVersion.(string))
		}
		config.ScramSHAVersion = scramSHAVersion.(string)
	} else if !ok && createOperation {
		config.ScramSHAVersion = SCRAMSHA512
	}

	if caBundle, ok := data.GetOk("ca_bundle"); ok {
		_, err := base64.StdEncoding.DecodeString(caBundle.(string))
		if err != nil {
			return nil, fmt.Errorf("unable to decode ca_bundle: %s", err)
		}
		config.CABundle = caBundle.(string)
	}

	if certificate, ok := data.GetOk("certificate"); ok {
		_, err := base64.StdEncoding.DecodeString(certificate.(string))
		if err != nil {
			return nil, fmt.Errorf("unable to decode certificate: %s", err)
		}
		config.Certificate = certificate.(string)
	}

	if certificateKey, ok := data.GetOk("certificate_key"); ok {
		_, err := base64.StdEncoding.DecodeString(certificateKey.(string))
		if err != nil {
			return nil, fmt.Errorf("unable to decode certificate key: %s", err)
		}
		config.CertificateKey = certificateKey.(string)
	}

	if config.Certificate != "" && config.CertificateKey == "" {
		return nil, fmt.Errorf("if certificate is set, certificate_key must also be set")
	}

	if config.CertificateKey != "" && config.Certificate == "" {
		return nil, fmt.Errorf("if certificate is set, certificate_key must also be set")
	}

	entry, err := logical.StorageEntryJSON(configStoragePath, config)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.reset()

	return nil, nil
}

func (b *kafkaBackend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, configStoragePath)

	if err == nil {
		b.reset()
	}

	return nil, err
}

// pathConfigExistenceCheck verifies if the configuration exists.
func (b *kafkaBackend) pathConfigExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	out, err := req.Storage.Get(ctx, req.Path)
	if err != nil {
		return false, fmt.Errorf("existence check failed: %w", err)
	}

	return out != nil, nil
}

func getConfig(ctx context.Context, s logical.Storage) (*kafkaConfig, error) {
	entry, err := s.Get(ctx, configStoragePath)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	config := new(kafkaConfig)
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, fmt.Errorf("error reading root configuration: %w", err)
	}

	// return the config, we are done
	return config, nil
}

// pathConfigHelpSynopsis summarizes the help text for the configuration
const pathConfigHelpSynopsis = `Configure the Kafka backend.`

// pathConfigHelpDescription describes the help text for the configuration
const pathConfigHelpDescription = `
The Kafka secret backend requires credentials for managing
JWTs issued to users working with the products API.

You must sign up with a username and password and
specify the Kafka address for the products API
before using this secrets backend.
`
