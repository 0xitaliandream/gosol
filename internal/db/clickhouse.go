// internal/db/clickhouse.go

package db

import (
	"context"
	"fmt"
	"gosol/internal/models"
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

func (db *Database) ListTransactionsForDetailsProcessing(ctx context.Context) ([]models.Transaction, error) {
	query := `
		SELECT t.id,
		       t.signature,
		FROM transactions AS t
		LEFT JOIN transactions_detail_job AS j FINAL
		       ON t.id = j.transaction_id
		WHERE 
			t.transaction_detail_id = toUUID('00000000-0000-0000-0000-000000000000')
			AND 
			(
				j.transaction_id IS NULL
				OR
				(
					j.last_enqueued_at < (now() - INTERVAL '1 HOUR')
					AND j.last_processed_at IS NULL
				)
			
			)
	`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query transactions for details processing: %w", err)
	}
	defer rows.Close()

	var txs []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		if err := rows.Scan(
			&tx.ID,
			&tx.Signature,
		); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return txs, nil
}

func (db *Database) SaveTransactionDetails(ctx context.Context, tx *models.TransactionDetail) error {
	stmt, err := db.conn.PrepareBatch(ctx, `
        INSERT INTO transactions_details (
			id,
			transaction_id,
            accounts,
            compute_units_consumed,
            fee,
            err,
            log_messages,
            instructions,
            version
        ) VALUES (
            ?,?,?,?,?,?,?,?,?
        )`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}

	// Create a map of token balances by account index
	preTokenBalancesByAccount := make(map[int64][]models.TokenBalance)
	for _, balance := range tx.PreTokenBalances {
		preTokenBalancesByAccount[balance.AccountIndex] = append(
			preTokenBalancesByAccount[balance.AccountIndex],
			balance,
		)
	}

	postTokenBalancesByAccount := make(map[int64][]models.TokenBalance)
	for _, balance := range tx.PostTokenBalances {
		postTokenBalancesByAccount[balance.AccountIndex] = append(
			postTokenBalancesByAccount[balance.AccountIndex],
			balance,
		)
	}

	// Transform accounts with combined balances
	accountsTuples := make([][]any, len(tx.Accounts))
	for i, account := range tx.Accounts {
		// Get token balances for this account
		preTokens := preTokenBalancesByAccount[int64(i)]
		postTokens := postTokenBalancesByAccount[int64(i)]

		// Create a map to match pre and post balances by mint
		type combinedBalance struct {
			owner          string
			tokenProgramId string
			decimals       int64
			preAmount      string
			postAmount     string
		}
		tokenBalances := make(map[string]*combinedBalance) // mint -> balance

		// Add pre token balances
		for _, preBalance := range preTokens {
			tokenBalances[preBalance.Mint] = &combinedBalance{
				owner:          preBalance.Owner,
				tokenProgramId: preBalance.ProgramId,
				decimals:       preBalance.UiTokenAmount.Decimals,
				preAmount:      preBalance.UiTokenAmount.UiAmountString,
			}
		}

		// Add post token balances
		for _, postBalance := range postTokens {
			if bal, exists := tokenBalances[postBalance.Mint]; exists {
				bal.postAmount = postBalance.UiTokenAmount.UiAmountString
			} else {
				tokenBalances[postBalance.Mint] = &combinedBalance{
					owner:          postBalance.Owner,
					tokenProgramId: postBalance.ProgramId,
					decimals:       postBalance.UiTokenAmount.Decimals,
					postAmount:     postBalance.UiTokenAmount.UiAmountString,
				}
			}
		}

		// Convert map to array of tuples
		tokenBalanceTuples := make([][]any, 0, len(tokenBalances))
		for mint, balance := range tokenBalances {
			tokenBalanceTuples = append(tokenBalanceTuples, []any{
				mint,
				balance.owner,
				balance.tokenProgramId,
				balance.decimals,
				balance.preAmount,
				balance.postAmount,
			})
		}

		// Create the complete account tuple with SOL balances
		accountsTuples[i] = []any{
			account.Pubkey,
			account.Signer,
			account.Writable,
			tx.PreSolBalances[i],
			tx.PostSolBalances[i],
			tokenBalanceTuples,
		}
	}

	// Transform instructions with nested inner instructions
	instructions := make([][]any, len(tx.Instructions))
	innerInstructionsMap := make(map[uint64][]models.Instruction)
	for _, innerInst := range tx.InnerInstructions {
		innerInstructionsMap[innerInst.Index] = innerInst.Instructions
	}

	for i, inst := range tx.Instructions {
		var parsedType, parsedAmount, parsedAuthority, parsedDestination,
			parsedSource, parsedMint, parsedTokenAmountAmount string
		var parsedTokenAmountDecimals int64
		var parsedTokenAmountUiAmountString string

		if inst.Parsed != nil {
			parsedType = inst.Parsed.Type
			parsedAmount = inst.Parsed.Info.Amount
			parsedAuthority = inst.Parsed.Info.Authority
			parsedDestination = inst.Parsed.Info.Destination
			parsedSource = inst.Parsed.Info.Source
			parsedMint = inst.Parsed.Info.Mint
			if inst.Parsed.Info.TokenAmount != nil {
				parsedTokenAmountAmount = inst.Parsed.Info.TokenAmount.Amount
				parsedTokenAmountDecimals = inst.Parsed.Info.TokenAmount.Decimals
				parsedTokenAmountUiAmountString = inst.Parsed.Info.TokenAmount.UiAmountString
			}
		}

		stackHeight := uint64(0)
		if inst.StackHeight != nil {
			stackHeight = *inst.StackHeight
		}

		innerInsts := make([][]any, 0)
		if inners, exists := innerInstructionsMap[uint64(i)]; exists {
			for _, inner := range inners {
				var innerParsedType, innerParsedAmount, innerParsedAuthority,
					innerParsedDestination, innerParsedSource, innerParsedMint,
					innerParsedTokenAmountAmount string
				var innerParsedTokenAmountDecimals int64
				var innerParsedTokenAmountUiAmountString string

				if inner.Parsed != nil {
					innerParsedType = inner.Parsed.Type
					innerParsedAmount = inner.Parsed.Info.Amount
					innerParsedAuthority = inner.Parsed.Info.Authority
					innerParsedDestination = inner.Parsed.Info.Destination
					innerParsedSource = inner.Parsed.Info.Source
					innerParsedMint = inner.Parsed.Info.Mint
					if inner.Parsed.Info.TokenAmount != nil {
						innerParsedTokenAmountAmount = inner.Parsed.Info.TokenAmount.Amount
						innerParsedTokenAmountDecimals = inner.Parsed.Info.TokenAmount.Decimals
						innerParsedTokenAmountUiAmountString = inner.Parsed.Info.TokenAmount.UiAmountString
					}
				}

				innerStackHeight := uint64(0)
				if inner.StackHeight != nil {
					innerStackHeight = *inner.StackHeight
				}

				innerInsts = append(innerInsts, []any{
					inner.Accounts,
					inner.Data,
					inner.ProgramId,
					inner.Program,
					innerParsedType,
					innerParsedAmount,
					innerParsedAuthority,
					innerParsedDestination,
					innerParsedSource,
					innerParsedMint,
					innerParsedTokenAmountAmount,
					innerParsedTokenAmountDecimals,
					innerParsedTokenAmountUiAmountString,
					innerStackHeight,
				})
			}
		}

		instructions[i] = []any{
			inst.Accounts,
			inst.Data,
			inst.ProgramId,
			inst.Program,
			parsedType,
			parsedAmount,
			parsedAuthority,
			parsedDestination,
			parsedSource,
			parsedMint,
			parsedTokenAmountAmount,
			parsedTokenAmountDecimals,
			parsedTokenAmountUiAmountString,
			stackHeight,
			innerInsts,
		}
	}

	// Add row to batch
	err = stmt.Append(
		tx.ID,
		tx.TransactionID,
		accountsTuples,
		tx.ComputeUnitsConsumed,
		tx.Fee,
		tx.Err,
		tx.LogMessages,
		instructions,
		tx.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to append values to batch: %w", err)
	}

	// Execute the batch
	err = stmt.Send()
	if err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	return nil
}

func (db *Database) SetTransactionDetailId(ctx context.Context, transactionID string, detailID string) error {
	query := `
        ALTER TABLE transactions UPDATE transaction_detail_id = $2
        WHERE id = $1
    `

	err := db.conn.Exec(ctx, query, transactionID, detailID)
	if err != nil {
		return fmt.Errorf("update transaction detail id: %w", err)
	}

	return nil
}
