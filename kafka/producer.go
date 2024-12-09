package kafka

import (
	"github.com/gitferry/bamboo/log"

	"github.com/Shopify/sarama"
)

type KafkaProducer struct {
	producer sarama.AsyncProducer
	topic    string
}

func NewKafkaProducer(brokers []string, topic string) (*KafkaProducer, error) {
	// 配置生产者
	config := sarama.NewConfig()
	config.Producer.Retry.Max = 3 // 设置重试次数
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	// 创建生产者
	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("消息队列生产者创建失败{%v} --- ip+port{%v},topic{%v}", err, brokers, topic)
		return nil, err
	}
	log.Infof("消息队列生产者创建成功ip+port{%v},topic{%v}", brokers, topic)

	return &KafkaProducer{
		producer: producer,
		topic:    topic,
	}, nil
}

// SendMessage 异步发送消息
func (kp *KafkaProducer) SendMessage(message string) {
	kp.producer.Input() <- &sarama.ProducerMessage{
		Topic: kp.topic,
		Value: sarama.StringEncoder(message),
	}
}

// Close 关闭生产者
func (kp *KafkaProducer) Close() error {
	return kp.producer.Close()
}
