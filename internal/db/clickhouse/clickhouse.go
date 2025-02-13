package clickhouse

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/sirupsen/logrus"
)

type Database struct {
	conn clickhouse.Conn
	log  *logrus.Logger
}

type Config struct {
	Hosts    []string
	Database string
	Username string
	Password string
	Debug    bool
}

func NewDatabase(cfg Config, log *logrus.Logger) (*Database, error) {
	log.Infof("Connessione a ClickHouse: %v", cfg)
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Hosts,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialContext: func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		},
		Debug: cfg.Debug,
		Debugf: func(format string, v ...any) {
			log.Debugf(format, v...)
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:          time.Second * 30,
		MaxOpenConns:         5,
		MaxIdleConns:         5,
		ConnMaxLifetime:      time.Duration(10) * time.Minute,
		ConnOpenStrategy:     clickhouse.ConnOpenInOrder,
		BlockBufferSize:      10,
		MaxCompressionBuffer: 10240,
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "gosol", Version: "0.1"},
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("errore nella connessione a ClickHouse: %w", err)
	}

	// Test della connessione
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("errore nel ping a ClickHouse: %w", err)
	}

	return &Database{
		conn: conn,
		log:  log,
	}, nil
}

// Close chiude la connessione al database
func (db *Database) Close() error {
	return db.conn.Close()
}

// GetConn restituisce la connessione raw al database
func (db *Database) GetConn() clickhouse.Conn {
	return db.conn
}
