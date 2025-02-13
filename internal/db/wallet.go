package db

import (
	"context"
	"fmt"
	"gosol/internal/models"
	"time"
)

func (db *Database) InsertWallet(ctx context.Context, w *models.Wallet) error {

	w.CreatedAt = time.Now().UTC()
	w.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO wallets
	`

	batch, err := db.conn.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}

	err = batch.AppendStruct(w)
	if err != nil {
		return err
	}

	err = batch.Send()
	if err != nil {
		return err
	}

	return nil
}

// Esempio di metodo per ottenere un wallet
func (db *Database) GetWallet(ctx context.Context, id string, w *models.Wallet) error {
	query := `
        SELECT id, address, created_at, updated_at
        FROM wallets FINAL
        WHERE id = $1
        LIMIT 1
    `
	err := db.conn.QueryRow(ctx, query, id).Scan(&w.ID, &w.Address, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) UpdateWallet(ctx context.Context, w *models.Wallet) error {

	w.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO wallets 
	`

	batch, err := db.conn.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}

	err = batch.AppendStruct(w)
	if err != nil {
		return err
	}

	err = batch.Send()
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) GetWallets(ctx context.Context, w *[]models.Wallet) error {
	query := `
        SELECT id, address, created_at, updated_at
        FROM wallets FINAL
    `
	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var wallet models.Wallet
		if err := rows.Scan(&wallet.ID, &wallet.Address, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
			return err
		}
		*w = append(*w, wallet)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func (db *Database) GetWalletsToMonitor(ctx context.Context, w *[]models.Wallet) error {
	query := `
		SELECT w.id, w.address, w.created_at, w.updated_at
		FROM wallets AS w FINAL
		LEFT JOIN wallets_monitor_job AS j FINAL
			ON w.id = j.wallet_id
		WHERE 
			j.wallet_id IS NULL
			OR
			(j.last_enqueued_at < (now() - INTERVAL '1 HOUR') 
			AND j.last_processed_at IS NULL) OR
					( j.last_processed_at < (now() - INTERVAL '1 HOUR') )
	`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var wallet models.Wallet
		if err := rows.Scan(&wallet.ID, &wallet.Address, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
			return err
		}
		*w = append(*w, wallet)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func (db *Database) UpdateWalletMonitorTimestamp(ctx context.Context, w *models.Wallet, isConsumer bool) error {
	var query string

	if isConsumer {
		// Consumer: inserisce anche last_processed_at
		query = `
			INSERT INTO wallets_monitor_job 
				(wallet_id, last_enqueued_at, last_processed_at, updated_at)
			VALUES
				($1, now(), now(), now())
		`
	} else {
		// Producer: non inserisce last_processed_at
		query = `
			INSERT INTO wallets_monitor_job 
				(wallet_id, last_enqueued_at, updated_at)
			VALUES
				($1, now(), now())
		`
	}

	err := db.conn.Exec(ctx, query, w.ID)
	if err != nil {
		return fmt.Errorf("update wallet monitor timestamp: %w", err)
	}

	return nil
}
