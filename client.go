package kafka

import (
	"errors"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// kafkaClient creates an object storing
// the client.
type kafkaClient struct {
	*kafka.AdminClient
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

	c, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": config.BootstrapServers,
	})
	if err != nil {
		return nil, err
	}

	return &kafkaClient{c}, nil
}
