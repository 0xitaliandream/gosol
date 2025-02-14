package main

import (
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
	solana := solana.NewClient(cfg.SolanaRPCURL)

	signature := "uoPGBv46sX9Z3MSBdJkFXrFyoB5WQnc6HCU2Rw3ZD8a3LwiRNGZkNbtMwUye6yAJRJkNmTo5to6ks5Tu1hxck5n"

	tx, err := solana.GetTransaction(signature)
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
