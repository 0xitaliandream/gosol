package models

import "time"

type Transaction struct {
	ID                  string    `json:"id" ch:"id"`
	Sequence            int64     `json:"sequence" ch:"sequence"`
	WalletID            string    `json:"wallet_id" ch:"wallet_id"`
	Signature           string    `json:"signature" ch:"signature"`
	Slot                uint64    `json:"slot" ch:"slot"`
	BlockTime           uint64    `json:"blockTime" ch:"block_time"`
	Status              string    `json:"confirmationStatus" ch:"status"`
	Err                 any       `json:"err" ch:"err"`
	Memo                string    `json:"memo" ch:"memo"`
	TransactionDetailId string    `json:"transaction_detail_id" ch:"transaction_detail_id"`
	CreatedAt           time.Time `json:"created_at" ch:"created_at"`
	UpdatedAt           time.Time `json:"updated_at-" ch:"updated_at"`
}

// TransactionMessage represents the message that will be sent to RabbitMQ
type TransactionRabbitMessage struct {
	ID        string `json:"id"`
	Signature string `json:"signature"`
}
