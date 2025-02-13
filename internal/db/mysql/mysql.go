package mysql

import (
	"fmt"
	"log"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	db  *gorm.DB
	log *logrus.Logger
}

type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Debug    bool
}

// NewDatabase crea una nuova istanza del database MySQL
func NewDatabase(cfg Config, log *logrus.Logger) (*Database, error) {
	log.Infof("Connessione a MySQL: %+v", cfg)

	// Costruisce la stringa di connessione
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	// Configura GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// Apre la connessione
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("errore nella connessione a MySQL: %w", err)
	}

	// Configura il pool di connessioni
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("errore nell'accesso al database SQL: %w", err)
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &Database{
		db:  db,
		log: log,
	}, nil
}

// Close chiude la connessione al database
func (db *Database) Close() error {
	sqlDB, err := db.db.DB()
	if err != nil {
		return fmt.Errorf("errore nell'accesso al database SQL: %w", err)
	}
	return sqlDB.Close()
}

// GetDB restituisce l'istanza di GORM DB
func (db *Database) GetDB() *gorm.DB {
	return db.db
}

func (db *Database) FirstMigration() error {
	// Migrazione in ordine corretto
	db.log.Println("Starting database tables...")

	err := db.GetDB().AutoMigrate(
		&Wallet{},
		&Transaction{},
		&WalletMonitorJob{},
		&TransactionDetailJob{},
	)
	if err != nil {
		log.Fatalf("Errore durante la migrazione: %v", err)
	}

	return nil
}
