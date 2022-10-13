package mq

import (
	"errors"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	pkgerr "github.com/pkg/errors"
)

func NewSyncProducer(brokers []string) (sarama.SyncProducer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
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

func NewConsumerGroup(group string, brokers []string) (sarama.ConsumerGroup, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Offsets.AutoCommit.Interval = time.Millisecond * 3
	//cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	return sarama.NewConsumerGroup(brokers, group, cfg)

}

type TopicConsumer struct {
	wg       *sync.WaitGroup
	cli      sarama.Consumer
	partions map[int32]sarama.PartitionConsumer
	msg      chan *sarama.ConsumerMessage
	err      chan *ErrorKafa
}

func (pc *TopicConsumer) Consumer() sarama.Consumer {
	return pc.cli
}

var (
	ErrNotParentConsumer = errors.New("not has parent consumer")
)

//NewConsumerAllPartitions's offset can be set as sarama.OffsetNewest,saramaOffsetOldest. dynamiclly add new partion is not supported
func NewConsumerAllPartitions(brokers []string, topic string, offset int64) (*TopicConsumer, error) {
	cfg := sarama.NewConfig()

	cfg.Consumer.Offsets.AutoCommit.Enable = true
	cfg.Consumer.IsolationLevel = sarama.ReadCommitted
	cfg.Consumer.Return.Errors = true

	c, err := sarama.NewConsumer(brokers, cfg)
	if err != nil {
		return nil, err
	}

	tc := &TopicConsumer{
		wg:  new(sync.WaitGroup),
		msg: make(chan *sarama.ConsumerMessage),
		cli: c,
		err: make(chan *ErrorKafa),
	}
	partitions, err := c.Partitions(topic)
	if err != nil {
		return nil, err
	}
	pcs := make(map[int32]sarama.PartitionConsumer)
	for _, p := range partitions {

		pc, err := c.ConsumePartition(topic, p, offset)
		if err != nil {
			return nil, err
		}
		pcs[p] = pc
		go func(p int32, pc sarama.PartitionConsumer) {
			go func() {
				for m := range pc.Messages() {
					tc.msg <- m
				}
			}()

			for err := range pc.Errors() {
				tc.err <- &ErrorKafa{
					partion: p,
					err:     err,
				}
			}

		}(p, pc)
	}
	tc.partions = pcs

	return tc, nil

}

func (tc *TopicConsumer) StartConsume(handler func(m *sarama.ConsumerMessage) error, errHandler func(err ErrorKafa)) {
	tc.wg.Add(2)
	go func() {

		for m := range tc.msg {
			go handler(m)
		}
		tc.wg.Done()
	}()
	go func() {
		for err := range tc.err {
			go errHandler(*err)
		}
		tc.wg.Done()
	}()

}

func (tc *TopicConsumer) Stop() {
	for _, pc := range tc.partions {
		pc.Close()
	}
	tc.cli.Close()
	tc.wg.Wait()
}

type ErrorKafa struct {
	partion int32
	err     error
}

func (ek ErrorKafa) Error() string {
	return ek.err.Error()
}

func (ek ErrorKafa) RootError() error {
	return ek.err
}

func (ek ErrorKafa) Partion() int32 {
	return ek.partion
}
