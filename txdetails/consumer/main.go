package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gosol/config"
	"gosol/internal/db"
	"gosol/internal/models"
	"gosol/internal/solana"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type TransactionMessage struct {
	ID        string `json:"id"`
	Signature string `json:"signature"`
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	cfg     *config.TxDetailsConfig
	log     *logrus.Logger
	db      *db.Database
	solana  *solana.Client
}

func NewConsumer(cfg *config.TxDetailsConfig, db *db.Database, solana *solana.Client, log *logrus.Logger) (*Consumer, error) {
	conn, err := amqp.Dial(cfg.RabbitURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	consumer := &Consumer{
		conn:    conn,
		channel: ch,
		cfg:     cfg,
		log:     log,
		db:      db,
		solana:  solana,
	}

	if err := consumer.setup(); err != nil {
		consumer.Close()
		return nil, err
	}

	return consumer, nil
}

func (c *Consumer) setup() error {
	// Setup dell'exchange principale
	err := c.channel.ExchangeDeclare(
		c.cfg.ExchangeName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	// Setup dell'exchange per i delayed messages
	err = c.channel.ExchangeDeclare(
		c.cfg.ExchangeName+".delayed",
		"x-delayed-message",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		amqp.Table{
			"x-delayed-type": "direct",
		},
	)
	if err != nil {
		return err
	}

	// Setup della coda principale
	q, err := c.channel.QueueDeclare(
		c.cfg.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	// Binding della coda all'exchange principale
	err = c.channel.QueueBind(
		q.Name,             // queue name
		c.cfg.RoutingKey,   // routing key
		c.cfg.ExchangeName, // exchange
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Binding della coda all'exchange delayed
	return c.channel.QueueBind(
		q.Name,                        // queue name
		c.cfg.RoutingKey,              // routing key
		c.cfg.ExchangeName+".delayed", // exchange
		false,
		nil,
	)
}

func (c *Consumer) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Consumer) scheduleRetry(msg models.TransactionRabbitMessage, retryCount int) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	delay := time.Duration(retryCount*5000) * time.Millisecond // Incrementa il delay ad ogni retry
	if delay > 30000*time.Millisecond {                        // Max delay di 30 secondi
		delay = 30000 * time.Millisecond
	}

	return c.channel.Publish(
		c.cfg.ExchangeName+".delayed", // exchange
		c.cfg.RoutingKey,              // routing key
		false,                         // mandatory
		false,                         // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Headers: amqp.Table{
				"x-delay":     int32(delay.Milliseconds()),
				"retry-count": retryCount + 1,
			},
			Timestamp: time.Now(),
		},
	)
}

func (c *Consumer) processMessage(ctx context.Context, delivery amqp.Delivery) error {
	var msg models.TransactionRabbitMessage
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	c.log.WithFields(logrus.Fields{
		"tx_id":     msg.ID,
		"signature": msg.Signature,
	}).Info("Processing transaction details")

	// Get transaction details from Solana
	txDetail, err := c.solana.GetTransaction(msg.Signature)
	if err != nil {
		retryCount := 0
		if count, ok := delivery.Headers["retry-count"].(int32); ok {
			retryCount = int(count)
		}

		if retryCount < 3 { // Max 3 retries
			if err := c.scheduleRetry(msg, retryCount); err != nil {
				return fmt.Errorf("failed to schedule retry: %w", err)
			}
			return fmt.Errorf("transaction not found, scheduled retry: %w", err)
		}
		return fmt.Errorf("max retries reached for transaction: %w", err)
	}

	txDetail.ID = uuid.New().String()
	txDetail.TransactionID = msg.ID

	// Save to database
	if err := c.db.SaveTransactionDetails(ctx, txDetail); err != nil {
		return fmt.Errorf("failed to save transaction details: %w", err)
	}

	if err := c.db.SetTransactionDetailId(ctx, msg.ID, txDetail.ID); err != nil {
		c.log.Errorf("Failed to set transaction detail id: %v", err)
	}

	if err := c.db.UpdateTransactionProcessingTimestamp(ctx, msg.ID, true); err != nil {
		c.log.Errorf("Failed to update processing timestamp: %v", err)
	}

	c.log.WithFields(logrus.Fields{
		"tx_id":     msg.ID,
		"signature": msg.Signature,
	}).Info("Successfully processed transaction details")

	return nil
}

func (c *Consumer) Start(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.cfg.QueueName, // queue
		"",              // consumer
		false,           // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		return err
	}

	c.log.Info("Starting transaction details consumer")

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery := <-msgs:
			if err := c.processMessage(ctx, delivery); err != nil {
				c.log.Errorf("Error processing message: %v", err)
				delivery.Nack(false, true) // requeue on error
			} else {
				delivery.Ack(false)
				c.log.Info("Message processed successfully")
			}
		}
	}
}

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.LoadTxDetailsConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.NewDatabase(db.Config{
		Hosts:    cfg.BaseConfig.DBHosts,
		Database: cfg.BaseConfig.DBName,
		Username: cfg.BaseConfig.DBUser,
		Password: cfg.BaseConfig.DBPassword,
		Debug:    cfg.BaseConfig.DBDebug,
	}, log)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	solanaClient := solana.NewClient(cfg.SolanaRPCURL)

	consumer, err := NewConsumer(cfg, database, solanaClient, log)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info("Shutting down consumer...")
		cancel()
	}()

	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("Consumer error: %v", err)
	}
}
