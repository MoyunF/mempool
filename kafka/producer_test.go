package kafka

import (
	"testing"

	"github.com/gitferry/bamboo/log"
)

// TestNewKafkaProducer 测试 KafkaProducer 的创建
func TestNewKafkaProducer(t *testing.T) {

	brokers := []string{"8.130.35.70:9092"}
	topic := "mempool"
	_, err := NewKafkaProducer(brokers, topic)
	if err != nil {
		t.Fatalf("Expected no error, but got %v", err)
	}

	log.Infof("Kafka producer created successfully with brokers: %v", brokers)
}

// // TestSendMessage 测试 SendMessage 方法
// func TestSendMessage(t *testing.T) {
// 	// 使用 mocks 创建一个模拟的 Kafka 生产者
// 	mockBroker := mocks.NewBroker(t)
// 	mockBroker.Start() 	
// 	defer mockBroker.Close()

// 	brokers := []string{mockBroker.Addr()}
// 	topic := "test-topic"

// 	producer, err := NewKafkaProducer(brokers, topic)
// 	if err != nil {
// 		t.Fatalf("Expected no error, but got %v", err)
// 	}

// 	// 创建一个通道来接收发送的消息
// 	mockProducer := producer.producer.(*mocks.AsyncProducer)
// 	mockProducer.ExpectSendMessageAndSucceed()

// 	// 发送一条消息
// 	message := "Test Message"
// 	producer.SendMessage(message)

// 	// 验证消息是否被成功发送
// 	mockProducer.AssertExpectations(t)

// 	log.Infof("Message sent successfully: %s", message)
// }

// // TestClose 测试 Close 方法
// func TestClose(t *testing.T) {
// 	// 使用 mocks 创建一个模拟的 Kafka 生产者
// 	mockBroker := mocks.NewBroker(t)
// 	mockBroker.Start()
// 	defer mockBroker.Close()

// 	brokers := []string{mockBroker.Addr()}
// 	topic := "test-topic"

// 	producer, err := NewKafkaProducer(brokers, topic)
// 	if err != nil {
// 		t.Fatalf("Expected no error, but got %v", err)
// 	}

// 	// 测试 Close 方法
// 	err = producer.Close()
// 	if err != nil {
// 		t.Fatalf("Expected no error, but got %v", err)
// 	}

// 	log.Infof("Kafka producer closed successfully")
// }
