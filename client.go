package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/IBM/sarama"
)

// kafkaClient creates an object storing
// the client.
type kafkaClient struct {
	sarama.ClusterAdmin
}

func createTLSConfiguration(tlsSkipVerify *bool, certFile *string, keyFile *string, caFile *string) (t *tls.Config) {
	t = &tls.Config{
		InsecureSkipVerify: *tlsSkipVerify,
	}
	if *certFile != "" && *keyFile != "" && *caFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := os.ReadFile(*caFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: *tlsSkipVerify,
		}
	}
	return t
}

// newClient creates a new client to access Kafka
// and exposes it for any secrets or roles to use.
func newClient(config *kafkaConfig) (*kafkaClient, error) {
	if config == nil {
		return nil, errors.New("client configuration was nil")
	}

	if config.Username == "" {
		return nil, errors.New("client username was not defined")
	}

	if config.Password == "" {
		return nil, errors.New("client password was not defined")
	}

	if config.BootstrapServers == "" {
		return nil, errors.New("kafka bootstrap servers was not defined")
	}

	tlsInsecureSkipVerify := false
	cert := "/secrets/kafka.pem"
	certKey := "/secrets/kafka-key.pem"
	ca := "/secrets/ca.pem"
	brokers := strings.Split(config.BootstrapServers, ",")

	kafkaConf := sarama.NewConfig()
	kafkaConf.Version = sarama.V3_5_1_0
	kafkaConf.ClientID = "vault_sasl_scram_client"
	kafkaConf.Metadata.Full = true
	kafkaConf.Net.SASL.Enable = true
	kafkaConf.Net.SASL.User = config.Username
	kafkaConf.Net.SASL.Password = config.Password
	kafkaConf.Net.SASL.Handshake = true
	kafkaConf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
	kafkaConf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	kafkaConf.Net.TLS.Enable = true
	kafkaConf.Net.TLS.Config = createTLSConfiguration(&tlsInsecureSkipVerify, &cert, &certKey, &ca)

	c, err := sarama.NewClusterAdmin(brokers, kafkaConf)
	if err != nil {
		return nil, err
	}

	return &kafkaClient{c}, nil
}
