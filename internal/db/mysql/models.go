package mysql

import (
	"time"
)

// Wallet rappresenta il modello per la tabella wallets
type Wallet struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement"`
	Address            string    `gorm:"type:varchar(255);not null"`
	IsLowerBoundSynced bool      `gorm:"default:false"`
	LowerSignatureBound string    `gorm:"type:varchar(255);null"`
	LowerTimestampBound uint64    `gorm:"type:bigint unsigned;null"`
	CreatedAt          time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`

	// Relazioni
	Transactions     []Transaction     `gorm:"foreignKey:WalletID"`
	WalletMonitorJob *WalletMonitorJob `gorm:"foreignKey:WalletID"`
}

// Transaction rappresenta il modello per la tabella transactions
type Transaction struct {
	ID                            uint      `gorm:"primaryKey;autoIncrement"`
	Sequence                      int64     `gorm:"type:bigint;not null"`
	WalletID                      uint      `gorm:"not null"`
	Signature                     string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	Slot                          uint64    `gorm:"type:bigint unsigned;not null"`
	BlockTime                     *uint64   `gorm:"type:bigint unsigned;null"`
	Status                        *string   `gorm:"type:varchar(50);null" json:"confirmationStatus"`
	ClickHouseTransactionDetailID *string   `gorm:"type:char(36);null"`
	CreatedAt                     time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt                     time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`

	// Relazioni
	Wallet               *Wallet               `gorm:"foreignKey:WalletID"`
	TransactionDetailJob *TransactionDetailJob `gorm:"foreignKey:TransactionID"`
}

// WalletMonitorJob rappresenta il modello per la tabella wallets_monitor_job
type WalletMonitorJob struct {
	ID              uint       `gorm:"primaryKey;autoIncrement"`
	WalletID        uint       `gorm:"not null;uniqueIndex"`
	LastEnqueuedAt  time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	LastProcessedAt *time.Time `gorm:"type:datetime"`
	CreatedAt       time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`

	// Relazioni
	Wallet *Wallet `gorm:"foreignKey:WalletID"`
}

// TransactionDetailJob rappresenta il modello per la tabella transactions_detail_job
type TransactionDetailJob struct {
	ID              uint       `gorm:"primaryKey;autoIncrement"`
	TransactionID   uint       `gorm:"not null;uniqueIndex"`
	LastEnqueuedAt  time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	LastProcessedAt *time.Time `gorm:"type:datetime"`
	CreatedAt       time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time  `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`

	// Relazioni
	Transaction *Transaction `gorm:"foreignKey:TransactionID"`
}
