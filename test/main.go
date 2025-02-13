package main

import (
	"context"
	"encoding/json"
	"fmt"
	"gosol/config"
	"gosol/internal/db"
	"gosol/internal/models"
	"gosol/internal/solana"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func testTransactionDetails(client *solana.Client, signature string, log *logrus.Logger, dbInstance *db.Database) error {
	// Get transaction details
	transaction, err := client.GetTransaction(signature)
	if err != nil {
		return fmt.Errorf("error retrieving transaction: %v", err)
	}

	jsonData, err := json.MarshalIndent(transaction, "", "")
	if err != nil {
		return fmt.Errorf("error in JSON serialization: %v", err)
	}

	// Print transaction details
	if transaction != nil {
		fmt.Printf("Transaction: %s\n", string(jsonData))
		transaction.ID = uuid.New().String()
		transaction.TransactionID = uuid.New().String()
	} else {
		log.Infof("Transaction not found")
	}

	if err := dbInstance.SaveTransactionDetails(context.Background(), transaction); err != nil {
		return fmt.Errorf("failed to save transaction details: %w", err)
	}

	return nil
}

func testSignatures(client *solana.Client, signature string, log *logrus.Logger, dbInstance *db.Database) error {
	// Get signatures
	signatures, err := client.GetSignaturesForAddress(signature, "", "", 1)
	if err != nil {
		return fmt.Errorf("error retrieving signatures: %v", err)
	}

	// Print signatures in JSON format
	if len(signatures) > 0 {
		jsonData, err := json.MarshalIndent(signatures, "", "")
		if err != nil {
			return fmt.Errorf("error in JSON serialization: %v", err)
		}
		fmt.Printf("Signatures: %s\n", string(jsonData))
	} else {
		log.Infof("Signatures not found")
	}

	return nil
}

// func testSwapTx(client *solana.Client, signature string, log *logrus.Logger, dbInstance *db.Database) error {
// 	rpcClient := rpc.New(rpc.MainNetBeta.RPC)

// 	// Replace with your actual transaction signature
// 	txSig := solana.MustSignatureFromBase58("2PRYeYqZ8YijAujdpC1x8ebWFkncETBYmbV1LyZdFHGHwRK3SaTwGfaK4mjBUPhCf1p1B68RMtZ7ffNyGjYWzSQz")

// 	// Specify the maximum transaction version supported
// 	var maxTxVersion uint64 = 0

// 	// Fetch the transaction data using the RPC client
// 	transaction, err := rpcClient.GetTransaction(
// 		context.TODO(),
// 		txSig,
// 		&rpc.GetTransactionOpts{
// 			Commitment:                     rpc.CommitmentConfirmed,
// 			MaxSupportedTransactionVersion: &maxTxVersion,
// 		},
// 	)
// 	if err != nil {
// 		log.Fatalf("Error fetching transaction: %s", err)
// 	}

// 	parser, err := solanaswapgo.NewTransactionParser(transaction)
// 	if err != nil {
// 		log.Fatalf("Error initializing transaction parser: %s", err)
// 	}

// 	// Parse the transaction to extract basic data
// 	transactionData, err := parser.ParseTransaction()
// 	if err != nil {
// 		log.Fatalf("Error parsing transaction: %s", err)
// 	}

// 	// Print the parsed transaction data
// 	marshalledData, _ := json.MarshalIndent(transactionData, "", "  ")
// 	fmt.Println(string(marshalledData))

// 	// Process and extract swap-specific data from the parsed transaction
// 	swapData, err := parser.ProcessSwapData(transactionData)
// 	if err != nil {
// 		log.Fatalf("Error processing swap data: %s", err)
// 	}

// 	// Print the parsed swap data
// 	marshalledSwapData, _ := json.MarshalIndent(swapData, "", "  ")
// 	fmt.Println(string(marshalledSwapData))

// 	return nil
// }

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.LoadMonitorConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbInstance, err := db.NewDatabase(db.Config{
		Hosts:    cfg.BaseConfig.DBHosts,
		Database: cfg.BaseConfig.DBName,
		Username: cfg.BaseConfig.DBUser,
		Password: cfg.BaseConfig.DBPassword,
		Debug:    cfg.BaseConfig.DBDebug,
	}, log)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbInstance.Close()

	newWallet := models.Wallet{
		ID:                 uuid.New().String(),
		Address:            "7Z8333333333333333333333333",
		IsLowerBoundSynced: false,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	fmt.Println("Wallet: ", newWallet)

	if err := dbInstance.InsertWallet(context.Background(), &newWallet); err != nil {
		log.Fatalf("Failed to save wallet: %v", err)
	}

	newWallet1 := models.Wallet{
		ID:                 uuid.New().String(),
		Address:            "7Z8333333333333333333333333",
		IsLowerBoundSynced: false,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	fmt.Println("Wallet: ", newWallet1)

	if err := dbInstance.InsertWallet(context.Background(), &newWallet1); err != nil {
		log.Fatalf("Failed to save wallet: %v", err)
	}

	time.Sleep(2 * time.Second)

	newWallet.Address = "TEST"

	if err := dbInstance.UpdateWallet(context.Background(), &newWallet); err != nil {
		log.Fatalf("Failed to update wallet: %v", err)
	}

	fmt.Println("Wallet: ", newWallet)

	wallets := []models.Wallet{}
	if err := dbInstance.GetWallets(context.Background(), &wallets); err != nil {
		log.Fatalf("Failed to get wallets: %v", err)
	}

	fmt.Println("Wallets: ", wallets)

	wallets2 := []models.Wallet{}
	if err := dbInstance.GetWalletsToMonitor(context.Background(), &wallets2); err != nil {
		log.Fatalf("Failed to get wallets: %v", err)
	}

	fmt.Println("Wallets: ", wallets2)

}
