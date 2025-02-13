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

// WalletMessage rappresenta il messaggio che verrà inviato a RabbitMQ

// Producer struttura principale del producer
type Producer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	db      *db.Database
	cfg     *config.MonitorConfig
	log     *logrus.Logger
}

// NewProducer crea una nuova istanza del producer
func NewProducer(cfg *config.MonitorConfig, db *db.Database, log *logrus.Logger) (*Producer, error) {
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
	walletsToMonitor := []models.Wallet{}
	err := p.db.GetWalletsToMonitor(ctx, &walletsToMonitor)
	if err != nil {
		return err
	}

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, wallet := range walletsToMonitor {
		msg := models.WalletRabbitMessage{
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

		if err := p.db.UpdateWalletMonitorTimestamp(ctx, &wallet, false); err != nil {
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
