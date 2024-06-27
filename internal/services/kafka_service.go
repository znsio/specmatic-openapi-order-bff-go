package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/znsio/specmatic-order-bff-go/internal/config"
	"github.com/znsio/specmatic-order-bff-go/internal/models"
)

func SendProductMessages(products []models.Product) error {
	cfg := config.GetConfig()

	// Create a new Kafka writer
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{cfg.KafkaBootstrapServers},
		Topic:    cfg.KafkaTopic,
		Balancer: &kafka.LeastBytes{},
	})
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, product := range products {
		productMessage := models.ProductMessage{
			ID:        product.ID,
			Name:      product.Name,
			Inventory: product.Inventory,
		}

		messageValue, err := json.Marshal(productMessage)
		if err != nil {
			return fmt.Errorf("error marshaling product message: %w", err)
		}

		err = w.WriteMessages(ctx, kafka.Message{
			Key:   []byte(strconv.Itoa(product.ID)),
			Value: messageValue,
		})
		if err != nil {
			return fmt.Errorf("error writing message to Kafka: %w", err)
		}
	}

	return nil
}
