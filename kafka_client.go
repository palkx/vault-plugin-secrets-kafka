package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
)

// kafkaClient creates an object storing
// the client.
type kafkaClient struct {
	sarama.ClusterAdmin
}

func createTLSConfiguration(tlsSkipVerify *bool, caFile *string, certFile *string, keyFile *string) (*tls.Config, error) {
	t := &tls.Config{}

	t.InsecureSkipVerify = *tlsSkipVerify

	if *caFile != "" {
		caCertsPEM, err := base64.StdEncoding.DecodeString(*caFile)
		if err != nil {
			return nil, fmt.Errorf("unable to decode ca_bundle: %s", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCertsPEM); !ok {
			return nil, fmt.Errorf("unable to append ca certificate")
		}
		t.RootCAs = caCertPool
	}

	if *certFile != "" && *keyFile != "" {
		certPem, err := base64.StdEncoding.DecodeString(*certFile)
		if err != nil {
			return nil, fmt.Errorf("unable to decode certificate: %s", err)
		}
		keyPem, err := base64.StdEncoding.DecodeString(*keyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to decode certificate key: %s", err)
		}
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("unable to create x509 key pair: %s", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}

	return t, nil
}

// newClient creates a new client to access Kafka
// and exposes it for any secrets or roles to use.
func newClient(config *kafkaConfig) (*kafkaClient, error) {
	if config == nil {
		return nil, errors.New("client configuration was nil")
	}

	if config.BootstrapServers == "" {
		return nil, errors.New("kafka bootstrap servers was not defined")
	}

	if config.Username == "" {
		return nil, errors.New("client username was not defined")
	}

	if config.Password == "" {
		return nil, errors.New("client password was not defined")
	}

	if config.Certificate != "" && config.CertificateKey == "" {
		return nil, errors.New("client certificate key is not defined, but certificate is provided")
	}

	if config.CertificateKey != "" && config.Certificate == "" {
		return nil, errors.New("client certificate is not defined, but certificate key is provided")
	}

	tlsSkipVerify := false
	brokers := strings.Split(config.BootstrapServers, ",")

	kafkaConf := sarama.NewConfig()
	kafkaConf.Version = sarama.V3_5_1_0
	kafkaConf.ClientID = "vault_sasl_scram_client"
	kafkaConf.Metadata.Full = true
	kafkaConf.Net.SASL.Enable = true
	kafkaConf.Net.SASL.User = config.Username
	kafkaConf.Net.SASL.Password = config.Password
	kafkaConf.Net.SASL.Handshake = true

	switch scramSHAVersion := config.ScramSHAVersion; scramSHAVersion {
	case SCRAMSHA256:
		kafkaConf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		kafkaConf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	case SCRAMSHA512:
		kafkaConf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		kafkaConf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	}

	if config.CABundle != "" || config.Certificate != "" {
		kafkaConf.Net.TLS.Enable = true
		tlsConfig, err := createTLSConfiguration(&tlsSkipVerify, &config.CABundle, &config.Certificate, &config.CertificateKey)
		if err != nil {
			return nil, err
		}
		kafkaConf.Net.TLS.Config = tlsConfig
	}

	c, err := sarama.NewClusterAdmin(brokers, kafkaConf)
	if err != nil {
		return nil, err
	}

	return &kafkaClient{c}, nil
}
