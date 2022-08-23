package mq

import (
	"time"

	"github.com/Shopify/sarama"
	pkgerr "github.com/pkg/errors"
)

func NewSyncProducer(brokers []string) (sarama.SyncProducer, error) {
	cfg := sarama.NewConfig()
	//cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForLocal     // Only wait for the leader to ack
	cfg.Producer.Compression = sarama.CompressionSnappy // Compress messages
	cfg.Producer.Flush.Frequency = 500 * time.Millisecond
	err := cfg.Validate()
	if err != nil {
		return nil, pkgerr.Wrapf(err, "with cfg:%+v", cfg)
	}
	sp, err := sarama.NewSyncProducer(brokers, cfg)
	return sp, pkgerr.Wrapf(err, "with cfg:%+v", cfg)
}
