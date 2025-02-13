package models

import "time"

type Wallet struct {
	ID                 string    `json:"id" ch:"id"`
	Address            string    `json:"address" ch:"address"`
	IsLowerBoundSynced bool      `json:"is_lower_bound_synced" ch:"is_lower_bound_synced"`
	CreatedAt          time.Time `json:"created_at" ch:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" ch:"updated_at"`
}

type WalletRabbitMessage struct {
	ID string `json:"id"`
}
