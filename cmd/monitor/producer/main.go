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

// WalletMessage rappresenta il messaggio che verrà inviato a RabbitMQ

// Producer struttura principale del producer
type Producer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	clickhouseDB *clickhouse.Database
	mysqlDB      *mysql.Database
	cfg          *config.MonitorConfig
	log          *logrus.Logger
}

// NewProducer crea una nuova istanza del producer
func NewProducer(cfg *config.MonitorConfig, clickhouseDB *clickhouse.Database, mysqlDB *mysql.Database, log *logrus.Logger) (*Producer, error) {
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

// setup configura gli exchange e le code necessarie
func (p *Producer) setup() error {
	// Setup dell'exchange principale
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

	// Setup dell'exchange per i delayed messages
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

	// Setup della coda principale
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

	// Binding della coda all'exchange principale
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

	// Binding della coda all'exchange delayed
	return p.channel.QueueBind(
		q.Name,                        // queue name
		p.cfg.RoutingKey,              // routing key
		p.cfg.ExchangeName+".delayed", // exchange
		false,
		nil,
	)
}

// Close chiude le connessioni
func (p *Producer) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// produceNewWallets pubblica i nuovi wallet sulla coda
func (p *Producer) produceNewWallets(ctx context.Context) error {
	var walletsToMonitor []mysql.Wallet

	if err := p.mysqlDB.GetDB().WithContext(ctx).
		Joins("LEFT JOIN wallet_monitor_jobs ON wallets.id = wallet_monitor_jobs.wallet_id").
		Where("wallet_monitor_jobs.wallet_id IS NULL OR " +
			"(wallet_monitor_jobs.last_enqueued_at < NOW() - INTERVAL 1 HOUR AND wallet_monitor_jobs.last_processed_at IS NULL) OR " +
			"wallet_monitor_jobs.last_processed_at < NOW() - INTERVAL 1 HOUR").
		Find(&walletsToMonitor).Error; err != nil {
		return fmt.Errorf("failed to get wallets to monitor: %w", err)
	}

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, wallet := range walletsToMonitor {
		msg := rabbitmq.WalletRabbitMessage{
			ID: wallet.ID,
		}

		body, err := json.Marshal(msg)
		if err != nil {
			p.log.Errorf("Failed to marshal wallet message: %v", err)
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
			p.log.Errorf("Failed to publish wallet message: %v", err)
			continue
		}

		if err := p.mysqlDB.GetDB().WithContext(ctx).
			Exec(`INSERT INTO wallet_monitor_jobs (wallet_id, last_enqueued_at, created_at, updated_at)
                  VALUES (?, NOW(), NOW(), NOW())
                  ON DUPLICATE KEY UPDATE last_enqueued_at = NOW(), updated_at = NOW()`,
				wallet.ID).Error; err != nil {
			p.log.Errorf("Failed to update monitor timestamp: %v", err)
			continue
		}

		p.log.WithFields(logrus.Fields{
			"wallet_id": wallet.ID,
		}).Info("Published new wallet to queue")
	}

	return nil
}

// Start avvia il producer
func (p *Producer) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.cfg.CheckInterval)
	defer ticker.Stop()

	p.log.Info("Starting wallet producer")

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := p.produceNewWallets(ctx); err != nil {
				p.log.Errorf("Error producing wallets: %v", err)
			}
		}
	}
}

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.LoadMonitorConfig()
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

	// Gestione graceful shutdown
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
