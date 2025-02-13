package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gosol/config"
	"gosol/internal/db"
	"gosol/internal/models"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Producer main producer structure
type Producer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	db      *db.Database
	cfg     *config.TxDetailsConfig
	log     *logrus.Logger
}

// NewProducer creates a new producer instance
func NewProducer(cfg *config.TxDetailsConfig, db *db.Database, log *logrus.Logger) (*Producer, error) {
	conn, err := amqp.Dial(cfg.RabbitURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	producer := &Producer{
		conn:    conn,
		channel: ch,
		db:      db,
		cfg:     cfg,
		log:     log,
	}

	if err := producer.setup(); err != nil {
		producer.Close()
		return nil, err
	}

	return producer, nil
}

// setup configures the exchanges and queues
func (p *Producer) setup() error {
	// Setup main exchange
	err := p.channel.ExchangeDeclare(
		p.cfg.ExchangeName,
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

	// Setup delayed exchange
	err = p.channel.ExchangeDeclare(
		p.cfg.ExchangeName+".delayed",
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

	// Setup main queue
	q, err := p.channel.QueueDeclare(
		p.cfg.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	// Bind queue to main exchange
	err = p.channel.QueueBind(
		q.Name,             // queue name
		p.cfg.RoutingKey,   // routing key
		p.cfg.ExchangeName, // exchange
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Bind queue to delayed exchange
	return p.channel.QueueBind(
		q.Name,                        // queue name
		p.cfg.RoutingKey,              // routing key
		p.cfg.ExchangeName+".delayed", // exchange
		false,
		nil,
	)
}

// Close closes all connections
func (p *Producer) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// produceTransactions publishes transactions that need processing to the queue
func (p *Producer) produceTransactions(ctx context.Context) error {

	// Get transactions with default tx_details_id that haven't been processed yet
	transactions, err := p.db.ListTransactionsForDetailsProcessing(ctx)
	if err != nil {
		return err
	}

	p.log.Infof("Found %d transactions for details processing", len(transactions))

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, tx := range transactions {
		msg := models.TransactionRabbitMessage{
			ID:        tx.ID,
			Signature: tx.Signature,
		}

		body, err := json.Marshal(msg)
		if err != nil {
			p.log.Errorf("Failed to marshal transaction message: %v", err)
			continue
		}

		err = p.channel.PublishWithContext(
			pubCtx,
			p.cfg.ExchangeName,
			p.cfg.RoutingKey,
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         body,
				Timestamp:    time.Now(),
			})

		if err != nil {
			p.log.Errorf("Failed to publish transaction message: %v", err)
			continue
		}

		// Update the transaction's processing timestamp to prevent reprocessing
		if err := p.db.UpdateTransactionProcessingTimestamp(ctx, tx.ID, false); err != nil {
			p.log.Errorf("Failed to update processing timestamp: %v", err)
			continue
		}

		p.log.WithFields(logrus.Fields{
			"tx_id":     tx.ID,
			"signature": tx.Signature,
		}).Info("Published transaction for details processing")
	}

	return nil
}

// Start starts the producer
func (p *Producer) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.cfg.CheckInterval)
	defer ticker.Stop()

	p.log.Info("Starting transaction details producer")

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := p.produceTransactions(ctx); err != nil {
				p.log.Errorf("Error producing transactions: %v", err)
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

	producer, err := NewProducer(cfg, database, log)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info("Shutting down producer...")
		cancel()
	}()

	if err := producer.Start(ctx); err != nil {
		log.Fatalf("Producer error: %v", err)
	}
}
