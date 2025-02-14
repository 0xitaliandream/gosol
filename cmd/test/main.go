package main

import (
	"context"
	"encoding/json"
	"fmt"
	"gosol/config"
	"gosol/internal/db/clickhouse"
	"gosol/internal/solana"

	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.LoadTxDetailsConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	solana := solana.NewClient(cfg.SolanaRPCURL, nil)

	signature := "2gzUfQheMcdH7EeztdVDsSzVN9X1EXdPo4YYRDxa8r7sXL7zV3j1nsNvXWV1z5qfVUeGDMPjw9YCVjH8hUW2c6Xc"

	tx, err := solana.GetTransaction(context.TODO(), signature)
	if err != nil {
		log.Fatalf("Failed to get transaction: %v", err)
	}

	parsedTx, err := clickhouse.ParseSolanaTransaction(tx)
	if err != nil {
		log.Fatalf("Failed to parse transaction: %v", err)
	}

	jsonTx, err := json.Marshal(parsedTx)
	if err != nil {
		log.Fatalf("Failed to marshal transaction: %v", err)
	}

	fmt.Println(string(jsonTx))
}
