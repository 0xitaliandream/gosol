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
	"gosol/internal/solana"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type TransactionBoundaries struct {
	newestSignature string
	lastSignature   string
	newestSequence  int64
	lastSequence    int64
}

type Consumer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	cfg          *config.MonitorConfig
	log          *logrus.Logger
	clickhouseDB *clickhouse.Database
	mysqlDB      *mysql.Database
	solana       *solana.Client
}

func NewConsumer(cfg *config.MonitorConfig, clickhouseDB *clickhouse.Database, mysqlDB *mysql.Database, solana *solana.Client, log *logrus.Logger) (*Consumer, error) {
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
		conn:         conn,
		channel:      ch,
		cfg:          cfg,
		log:          log,
		clickhouseDB: clickhouseDB,
		mysqlDB:      mysqlDB,
		solana:       solana,
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

func (c *Consumer) scheduleNextCheck(wallet rabbitmq.WalletRabbitMessage) error {
	body, err := json.Marshal(wallet)
	if err != nil {
		return err
	}

	// Pubblica il messaggio sull'exchange delayed con un delay di 5 minuti
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
				"x-delay": 30000, // 1 minuti in millisecondi
			},
			Timestamp: time.Now(),
		},
	)
}

func (c *Consumer) processMessage(ctx context.Context, delivery amqp.Delivery) error {
	var walletQueuedMessage rabbitmq.WalletRabbitMessage
	if err := json.Unmarshal(delivery.Body, &walletQueuedMessage); err != nil {
		return err
	}

	c.log.WithFields(logrus.Fields{
		"wallet_id": walletQueuedMessage.ID}).Info("Processing wallet")

	// Get wallet direttamente con GORM
	var wallet mysql.Wallet
	if err := c.mysqlDB.GetDB().WithContext(ctx).
		First(&wallet, walletQueuedMessage.ID).Error; err != nil {
		c.log.Errorf("Failed to get wallet: %v", err)
		return err
	}

	// Process wallet transactions
	if err := c.processWalletTransactions(ctx, &wallet); err != nil {
		c.log.Errorf("Failed to process wallet transactions: %v", err)
		return err
	}

	// Update monitor job timestamp usando NOW() di MySQL
	if err := c.mysqlDB.GetDB().WithContext(ctx).
		Exec(`UPDATE wallet_monitor_jobs 
              SET last_processed_at = NOW(), 
                  updated_at = NOW() 
              WHERE wallet_id = ?`,
			wallet.ID).Error; err != nil {
		c.log.Errorf("Failed to update monitor timestamp: %v", err)
		return err
	}

	// Schedule next check
	return c.scheduleNextCheck(walletQueuedMessage)
}

func (c *Consumer) processWalletTransactions(ctx context.Context, w *mysql.Wallet) error {
	boundaries := TransactionBoundaries{}
	err := c.getTransactionBoundaries(ctx, &boundaries, w)
	if err != nil {
		return err
	}

	return c.processTransactions(ctx, w, &boundaries)
}

func (c *Consumer) getTransactionBoundaries(ctx context.Context, boundaries *TransactionBoundaries, w *mysql.Wallet) error {
	// Get newest transaction (highest sequence)
	var newestTx mysql.Transaction
	if err := c.mysqlDB.GetDB().WithContext(ctx).
		Where("wallet_id = ?", w.ID).
		Order("sequence DESC").
		First(&newestTx).Error; err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to get newest transaction: %w", err)
	}

	if newestTx.Signature != "" {
		boundaries.newestSignature = newestTx.Signature
		boundaries.newestSequence = newestTx.Sequence
	}

	// Get oldest transaction (lowest sequence)
	var oldestTx mysql.Transaction
	if err := c.mysqlDB.GetDB().WithContext(ctx).
		Where("wallet_id = ?", w.ID).
		Order("sequence ASC").
		First(&oldestTx).Error; err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to get oldest transaction: %w", err)
	}

	if oldestTx.Signature != "" {
		boundaries.lastSignature = oldestTx.Signature
		boundaries.lastSequence = oldestTx.Sequence
	}

	return nil
}

func (c *Consumer) processTransactions(ctx context.Context, wallet *mysql.Wallet, boundaries *TransactionBoundaries) error {
	c.log.Infof("Processing wallet %s from %s to %s",
		wallet.Address, boundaries.lastSignature, boundaries.newestSignature)

	if !wallet.IsLowerBoundSynced {
		// Check for older transactions
		if hasOlder, err := c.checkForOlderTransactions(ctx, wallet, boundaries); err != nil {
			return fmt.Errorf("failed to check older transactions: %w", err)
		} else if hasOlder {
			if err := c.processBackwardTransactions(ctx, wallet, boundaries); err != nil {
				return err
			}
			return nil
		} else {
			wallet.IsLowerBoundSynced = true
			if err := c.mysqlDB.GetDB().WithContext(ctx).Save(wallet).Error; err != nil {
				return fmt.Errorf("failed to update wallet: %w", err)
			}
		}
	}

	// if boundaries.newestSignature == "" {
	// 	return nil
	// }
	// if hasNewer, err := c.checkForNewerTransactions(ctx, wallet, boundaries); err != nil {
	// 	return fmt.Errorf("failed to check newer transactions: %w", err)
	// } else if hasNewer {
	// 	if err := c.processForwardTransactions(ctx, wallet, boundaries); err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }

	return nil
}

func (c *Consumer) checkForOlderTransactions(ctx context.Context, w *mysql.Wallet, boundaries *TransactionBoundaries) (bool, error) {
	transactions, err := c.solana.GetSignaturesForAddress(ctx, w.Address, "", boundaries.lastSignature, 1)
	if err != nil {
		return false, err
	}
	return len(transactions) > 0, nil
}

func (c *Consumer) checkForNewerTransactions(ctx context.Context, w *mysql.Wallet, boundaries *TransactionBoundaries) (bool, error) {
	transactions, err := c.solana.GetSignaturesForAddress(ctx, w.Address, boundaries.newestSignature, "", 1)
	if err != nil {
		return false, fmt.Errorf("failed to check newer transactions: %w", err)
	}
	return len(transactions) > 0, nil
}

func (c *Consumer) processBackwardTransactions(ctx context.Context, w *mysql.Wallet, boundaries *TransactionBoundaries) error {
	before := boundaries.lastSignature
	index := boundaries.lastSequence

	for {
		c.log.Infof("BACK Index: %d", index)
		transactions, err := c.solana.GetSignaturesForAddress(ctx, w.Address, "", before, c.cfg.GetSignaturesLimitRpc)
		if err != nil {
			return fmt.Errorf("failed to get backward transactions: %w", err)
		}

		if len(transactions) == 0 {
			break
		}

		// Check if we've reached the LowerSignatureBound
		if w.LowerSignatureBound != "" {
			// Find the index of the bound signature
			boundIndex := -1
			for i, tx := range transactions {
				if tx.Signature == w.LowerSignatureBound {
					boundIndex = i
					break
				}
			}

			if boundIndex != -1 {
				// Save only transactions up to the bound (inclusive)
				if err := c.saveTransactions(ctx, w.ID, transactions[:boundIndex+1], &index, false); err != nil {
					return err
				}
				// We found the bound, mark as synced and exit
				w.IsLowerBoundSynced = true
				if err := c.mysqlDB.GetDB().WithContext(ctx).Save(w).Error; err != nil {
					return fmt.Errorf("failed to update wallet sync status: %w", err)
				}
				return nil
			}
		}

		// Check timestamp bound if no signature bound is set or not yet reached
		if w.LowerSignatureBound == "" && w.LowerTimestampBound > 0 {
			// Find the first transaction that's older than our bound
			boundIndex := -1
			for i, tx := range transactions {
				if tx.BlockTime != nil && *tx.BlockTime <= w.LowerTimestampBound {
					boundIndex = i
					break
				}
			}

			if boundIndex != -1 {
				// Save only transactions up to the bound (inclusive)
				if err := c.saveTransactions(ctx, w.ID, transactions[:boundIndex+1], &index, false); err != nil {
					return err
				}
				// We reached the timestamp bound, mark as synced and exit
				w.IsLowerBoundSynced = true
				if err := c.mysqlDB.GetDB().WithContext(ctx).Save(w).Error; err != nil {
					return fmt.Errorf("failed to update wallet sync status: %w", err)
				}
				return nil
			}
		}

		// If no bounds are hit, save all transactions and continue
		if err := c.saveTransactions(ctx, w.ID, transactions, &index, false); err != nil {
			return err
		}

		before = transactions[len(transactions)-1].Signature
	}

	// If we get here without finding any bounds, we've reached the end naturally
	w.IsLowerBoundSynced = true
	if err := c.mysqlDB.GetDB().WithContext(ctx).Save(w).Error; err != nil {
		return fmt.Errorf("failed to update wallet sync status: %w", err)
	}

	return nil
}

func (c *Consumer) processForwardTransactions(ctx context.Context, w *mysql.Wallet, boundaries *TransactionBoundaries) error {
	until := boundaries.newestSignature
	before := ""
	index := boundaries.newestSequence

	for {
		c.log.Infof("FORW Index: %d", index)
		transactions, err := c.solana.GetSignaturesForAddress(ctx, w.Address, until, before, c.cfg.GetSignaturesLimitRpc)
		if err != nil {
			return fmt.Errorf("failed to get forward transactions: %w", err)
		}

		if len(transactions) == 0 {
			break
		}

		if err := c.saveTransactions(ctx, w.ID, transactions, &index, true); err != nil {
			return err
		}

		before = transactions[len(transactions)-1].Signature
	}

	return nil
}

func (c *Consumer) saveTransactions(ctx context.Context, walletID uint, transactions []mysql.Transaction, index *int64, increment bool) error {
	for _, tx := range transactions {
		tx.WalletID = walletID
		tx.Sequence = *index

		if err := c.mysqlDB.GetDB().WithContext(ctx).Create(&tx).Error; err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}

		if increment {
			*index++
		} else {
			*index--
		}
	}
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

	c.log.Info("Starting wallet consumer")

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
				c.log.Info("Message processed and rescheduled successfully")
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

	solanaClient := solana.NewClient(cfg.SolanaRPCURL, nil)

	consumer, err := NewConsumer(cfg, clickhouseDB, mysqlDB, solanaClient, log)
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
