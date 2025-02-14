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
	"gosol/internal/db/clickhouse"
	"gosol/internal/db/mysql"
	"gosol/internal/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Producer main producer structure
type Producer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	clickhouseDB *clickhouse.Database
	mysqlDB      *mysql.Database
	cfg          *config.TxDetailsConfig
	log          *logrus.Logger
}

// NewProducer creates a new producer instance
func NewProducer(cfg *config.TxDetailsConfig, clickhouseDB *clickhouse.Database, mysqlDB *mysql.Database, log *logrus.Logger) (*Producer, error) {
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
		conn:         conn,
		channel:      ch,
		clickhouseDB: clickhouseDB,
		mysqlDB:      mysqlDB,
		cfg:          cfg,
		log:          log,
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

	var transactions []mysql.Transaction

	if err := p.mysqlDB.GetDB().WithContext(ctx).
		Joins("LEFT JOIN transaction_detail_jobs ON transactions.id = transaction_detail_jobs.transaction_id").
		Where("transactions.click_house_transaction_detail_id IS NULL").
		Where("(transaction_detail_jobs.transaction_id IS NULL OR " +
			"(transaction_detail_jobs.last_enqueued_at < NOW() - INTERVAL 1 HOUR AND transaction_detail_jobs.last_processed_at IS NULL))").
		Find(&transactions).Error; err != nil {
		return fmt.Errorf("failed to get transactions for details processing: %w", err)
	}

	p.log.Infof("Found %d transactions for details processing", len(transactions))

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, tx := range transactions {
		msg := rabbitmq.TransactionRabbitMessage{
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

		if err := p.mysqlDB.GetDB().WithContext(ctx).
			Exec(`INSERT INTO transaction_detail_jobs (transaction_id, last_enqueued_at, created_at, updated_at)
	  VALUES (?, NOW(), NOW(), NOW())
	  ON DUPLICATE KEY UPDATE last_enqueued_at = NOW(), updated_at = NOW()`,
				tx.ID).Error; err != nil {
			return fmt.Errorf("failed to update transaction processing timestamp: %w", err)
		}

		// p.log.WithFields(logrus.Fields{
		// 	"tx_id":     tx.ID,
		// 	"signature": tx.Signature,
		// }).Info("Published transaction for details processing")
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

	clickhouseDB, err := clickhouse.NewDatabase(clickhouse.Config{
		Hosts:    cfg.CHHosts,
		Database: cfg.CHDb,
		Username: cfg.CHUser,
		Password: cfg.CHPassword,
		Debug:    cfg.CHDebug,
	}, log)
	if err != nil {
		log.Fatalf("Errore nella connessione al database: %v", err)
	}
	defer clickhouseDB.Close()

	mysqlDB, err := mysql.NewDatabase(mysql.Config{
		Host:     cfg.MySQLHost,
		Port:     cfg.MySQLPort,
		Database: cfg.MySQLDb,
		Username: cfg.MySQLUser,
		Password: cfg.MySQLPassword,
		Debug:    cfg.MySQLDebug,
	}, log)
	if err != nil {
		log.Fatalf("Errore nella connessione al database: %v", err)
	}
	defer mysqlDB.Close()

	producer, err := NewProducer(cfg, clickhouseDB, mysqlDB, log)
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
