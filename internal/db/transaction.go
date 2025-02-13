package db

import (
	"context"
	"database/sql"
	"fmt"
	"gosol/internal/models"
	"time"

	"github.com/google/uuid"
)

func (db *Database) InsertTransaction(ctx context.Context, tx *models.Transaction) error {

	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = time.Now().UTC()
	tx.ID = uuid.New().String()

	query := `
		INSERT INTO transactions
	`

	batch, err := db.conn.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}

	err = batch.AppendStruct(tx)
	if err != nil {
		return err
	}

	err = batch.Send()
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) UpdateTransaction(ctx context.Context, tx *models.Transaction) error {

	tx.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO transactions
	`

	batch, err := db.conn.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}

	err = batch.AppendStruct(tx)
	if err != nil {
		return err
	}

	err = batch.Send()
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) GetLastTransaction(ctx context.Context, tx *models.Transaction, w *models.Wallet) error {
	query := `
		SELECT sequence, signature
		FROM transactions FINAL
		WHERE wallet_id = $1
		ORDER BY block_time ASC, slot ASC, sequence ASC
		LIMIT 1 BY wallet_id;
	`

	err := db.conn.QueryRow(ctx, query, w.ID).Scan(&tx.Sequence, &tx.Signature)
	if err != nil {
		if err == sql.ErrNoRows {
			// Clear the transaction fields to indicate no rows found
			tx.Sequence = 0
			tx.Signature = ""
			return nil
		}
		return err
	}

	return nil
}

func (db *Database) GetNewestTransaction(ctx context.Context, tx *models.Transaction, w *models.Wallet) error {
	query := `
		SELECT sequence, signature
		FROM transactions FINAL
		WHERE wallet_id = $1
		ORDER BY block_time DESC, slot DESC, sequence DESC
		LIMIT 1 BY wallet_id;
	`

	err := db.conn.QueryRow(ctx, query, w.ID).Scan(&tx.Sequence, &tx.Signature)
	if err != nil {
		if err == sql.ErrNoRows {
			// Clear the transaction fields to indicate no rows found
			tx.Sequence = 0
			tx.Signature = ""
			return nil
		}
		return err
	}

	return nil
}

func (db *Database) UpdateTransactionProcessingTimestamp(ctx context.Context, transactionID string, isConsumer bool) error {
	var query string

	if isConsumer {
		// Consumer: inserisce anche last_processed_at
		query = `
			INSERT INTO transactions_detail_job 
				(transaction_id, last_enqueued_at, last_processed_at, updated_at)
			VALUES
				($1, now(), now(), now())
		`
	} else {
		// Producer: non inserisce last_processed_at
		query = `
			INSERT INTO transactions_detail_job 
				(transaction_id, last_enqueued_at, updated_at)
			VALUES
				($1, now(), now())
		`
	}

	err := db.conn.Exec(ctx, query, transactionID)
	if err != nil {
		return fmt.Errorf("update transaction processing timestamp: %w", err)
	}

	return nil
}
