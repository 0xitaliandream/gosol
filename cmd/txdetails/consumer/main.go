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

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type TransactionMessage struct {
	ID        string `json:"id"`
	Signature string `json:"signature"`
}

type Consumer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	cfg          *config.TxDetailsConfig
	log          *logrus.Logger
	clickhouseDB *clickhouse.Database
	mysqlDB      *mysql.Database
	solana       *solana.Client
}

func NewConsumer(cfg *config.TxDetailsConfig, db *clickhouse.Database, mysqlDB *mysql.Database, solana *solana.Client, log *logrus.Logger) (*Consumer, error) {
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
		clickhouseDB: db,
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

func (c *Consumer) scheduleRetry(msg rabbitmq.TransactionRabbitMessage, retryCount int) error {
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
	var msg rabbitmq.TransactionRabbitMessage
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	c.log.WithFields(logrus.Fields{
		"tx_id":     msg.ID,
		"signature": msg.Signature,
	}).Info("Processing transaction details")

	// Get transaction details from Solana
	tx, err := c.solana.GetTransaction(msg.Signature)
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

	txDetail, err := clickhouse.ParseSolanaTransaction(tx)
	if err != nil {
		return fmt.Errorf("failed to build transaction details: %w", err)
	}

	txDetail.ID = uuid.New().String()
	txDetail.MysqlTransactionID = msg.ID

	if err := c.SaveTransactionDetails(ctx, &txDetail); err != nil {
		return fmt.Errorf("failed to save transaction details: %w", err)
	}

	if err := c.mysqlDB.GetDB().WithContext(ctx).
		Exec("UPDATE transactions SET click_house_transaction_detail_id = ?, updated_at = NOW() WHERE id = ?",
			txDetail.ID, msg.ID).Error; err != nil {
		return fmt.Errorf("failed to set transaction detail id: %w", err)
	}

	if err := c.mysqlDB.GetDB().WithContext(ctx).
		Exec(`INSERT INTO transaction_detail_jobs (transaction_id, last_processed_at, created_at, updated_at)
          VALUES (?, NOW(), NOW(), NOW())
          ON DUPLICATE KEY UPDATE last_processed_at = NOW(), updated_at = NOW()`,
			msg.ID).Error; err != nil {
		return fmt.Errorf("failed to update transaction processing timestamp: %w", err)
	}

	c.log.WithFields(logrus.Fields{
		"tx_id":     msg.ID,
		"signature": msg.Signature,
	}).Info("Successfully processed transaction details")

	return nil
}

func (c *Consumer) SaveTransactionDetails(ctx context.Context, tx *clickhouse.TransactionDetail) error {
	stmt, err := c.clickhouseDB.GetConn().PrepareBatch(ctx, `
        INSERT INTO transactions_details (
            id,
            mysql_transaction_id,
            accounts,
            compute_units_consumed,
            fee,
            err,
            log_messages,
            instructions,
            version,
            block_time
        ) VALUES (
            ?,?,?,?,?,?,?,?,?,?
        )`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}

	// Convert accounts to tuple format
	accountsTuples := make([][]interface{}, len(tx.Accounts))
	for i, account := range tx.Accounts {
		// Convert token balances to tuples
		tokenBalancesTuples := make([][]interface{}, len(account.TokenBalances))
		for j, balance := range account.TokenBalances {
			tokenBalancesTuples[j] = []interface{}{
				balance.Mint,
				balance.Owner,
				balance.TokenProgramId,
				balance.Decimals,
				balance.PreAmount,
				balance.PostAmount,
				balance.ChangeAmount,
			}
		}

		accountsTuples[i] = []interface{}{
			account.Pubkey,
			account.Signer,
			account.Writable,
			account.PreSolBalance,
			account.PostSolBalance,
			account.SolChange,
			tokenBalancesTuples,
		}
	}

	// Convert instructions to tuple format
	instructionsTuples := make([][]interface{}, len(tx.Instructions))
	for i, inst := range tx.Instructions {
		// Convert inner instructions to tuples
		innerInstructionsTuples := make([][]interface{}, len(inst.InnerInstructions))
		for j, inner := range inst.InnerInstructions {
			innerInstructionsTuples[j] = []interface{}{
				inner.Accounts,
				inner.Data,
				inner.ProgramId,
				inner.Program,
				inner.ParsedType,
				inner.ParsedAmount,
				inner.ParsedAuthority,
				inner.ParsedDestination,
				inner.ParsedSource,
				inner.ParsedMint,
				inner.ParsedTokenAmountAmount,
				inner.ParsedTokenAmountDecimals,
				inner.ParsedTokenAmountUiAmountString,
				inner.StackHeight,
			}
		}

		instructionsTuples[i] = []interface{}{
			inst.Accounts,
			inst.Data,
			inst.ProgramId,
			inst.Program,
			inst.ParsedType,
			inst.ParsedAmount,
			inst.ParsedAuthority,
			inst.ParsedDestination,
			inst.ParsedSource,
			inst.ParsedMint,
			inst.ParsedTokenAmountAmount,
			inst.ParsedTokenAmountDecimals,
			inst.ParsedTokenAmountUiAmountString,
			inst.StackHeight,
			innerInstructionsTuples,
		}
	}

	// Add row to batch
	err = stmt.Append(
		tx.ID,
		tx.MysqlTransactionID,
		accountsTuples,
		tx.ComputeUnitsConsumed,
		tx.Fee,
		tx.Err,
		tx.LogMessages,
		instructionsTuples,
		tx.Version,
		tx.BlockTime,
	)
	if err != nil {
		return fmt.Errorf("failed to append values to batch: %w", err)
	}

	// Execute the batch
	err = stmt.Send()
	if err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
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

	solana := solana.NewClient(cfg.SolanaRPCURL)

	consumer, err := NewConsumer(cfg, clickhouseDB, mysqlDB, solana, log)
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
