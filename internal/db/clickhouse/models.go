package clickhouse

import (
	"fmt"
	"gosol/internal/solana"

	"github.com/shopspring/decimal"
)

// TokenBalance rappresenta il bilancio di un token per un account
type TokenBalance struct {
	Mint           string          `json:"mint"`
	Owner          string          `json:"owner"`
	TokenProgramId string          `json:"token_program_id"`
	Decimals       int64           `json:"decimals"`
	PreAmount      decimal.Decimal `json:"pre_amount"`
	PostAmount     decimal.Decimal `json:"post_amount"`
	ChangeAmount   decimal.Decimal `json:"change_amount"`
}

// Account rappresenta un account con i suoi bilanci
type Account struct {
	Pubkey         string         `json:"pubkey"`
	Signer         bool           `json:"signer"`
	Writable       bool           `json:"writable"`
	PreSolBalance  uint64         `json:"pre_sol_balance"`
	PostSolBalance uint64         `json:"post_sol_balance"`
	SolChange      int64          `json:"sol_change"`
	TokenBalances  []TokenBalance `json:"token_balances"`
}

// Instruction rappresenta un'istruzione nella transazione
type InstructionDetail struct {
	Accounts                        []string            `json:"accounts"`
	Data                            string              `json:"data"`
	ProgramId                       string              `json:"program_id"`
	Program                         string              `json:"program"`
	ParsedType                      string              `json:"parsed_type"`
	ParsedAmount                    string              `json:"parsed_amount"`
	ParsedAuthority                 string              `json:"parsed_authority"`
	ParsedDestination               string              `json:"parsed_destination"`
	ParsedSource                    string              `json:"parsed_source"`
	ParsedMint                      string              `json:"parsed_mint"`
	ParsedTokenAmountAmount         string              `json:"parsed_token_amount_amount"`
	ParsedTokenAmountDecimals       int64               `json:"parsed_token_amount_decimals"`
	ParsedTokenAmountUiAmountString string              `json:"parsed_token_amount_ui_amount_string"`
	StackHeight                     uint64              `json:"stack_height"`
	InnerInstructions               []InstructionDetail `json:"inner_instructions"`
}

// TransactionDetail rappresenta i dettagli di una transazione come salvati in Clickhouse
type TransactionDetail struct {
	ID                   string              `json:"id"`
	MysqlTransactionID   uint                `json:"mysql_transaction_id"`
	BlockTime            int64               `json:"block_time"`
	Accounts             []Account           `json:"accounts"`
	ComputeUnitsConsumed uint64              `json:"compute_units_consumed"`
	Fee                  uint64              `json:"fee"`
	Err                  string              `json:"err"`
	LogMessages          []string            `json:"log_messages"`
	Instructions         []InstructionDetail `json:"instructions"`
	Version              uint64              `json:"version"`
}

// type TransactionDetail struct {
// 	ID                   string                    `json:"id"`
// 	MysqlTransactionID   uint                      `json:"mysql_transaction_id"`
// 	BlockTime            int64                     `json:"block_time"`
// 	Accounts             []solana.Account          `json:"accounts"`
// 	ComputeUnitsConsumed uint64                    `json:"compute_units_consumed"`
// 	Fee                  uint64                    `json:"fee"`
// 	Err                  string                    `json:"err"`
// 	LogMessages          []string                  `json:"log_messages"`
// 	PreTokenBalances     []solana.TokenBalance     `json:"pre_token_balances"`
// 	PostTokenBalances    []solana.TokenBalance     `json:"post_token_balances"`
// 	PreSolBalances       []uint64                  `json:"pre_sol_balances"`
// 	PostSolBalances      []uint64                  `json:"post_sol_balances"`
// 	Instructions         []solana.Instruction      `json:"instructions"`
// 	InnerInstructions    []solana.InnerInstruction `json:"inner_instructions"`
// 	Version              solana.TransactionVersion `json:"version"`
// }

func ParseSolanaTransaction(tx *solana.SolanaTransaction) (TransactionDetail, error) {
	// Create a map of token balances by account index
	preTokenBalancesByAccount := make(map[int64][]solana.TokenBalance)
	for _, balance := range tx.Meta.PreTokenBalances {
		preTokenBalancesByAccount[balance.AccountIndex] = append(
			preTokenBalancesByAccount[balance.AccountIndex],
			balance,
		)
	}

	postTokenBalancesByAccount := make(map[int64][]solana.TokenBalance)
	for _, balance := range tx.Meta.PostTokenBalances {
		postTokenBalancesByAccount[balance.AccountIndex] = append(
			postTokenBalancesByAccount[balance.AccountIndex],
			balance,
		)
	}

	// Transform accounts with combined balances
	accounts := make([]Account, len(tx.Transaction.Message.AccountKeys))
	for i, account := range tx.Transaction.Message.AccountKeys {
		// Get token balances for this account
		preTokens := preTokenBalancesByAccount[int64(i)]
		postTokens := postTokenBalancesByAccount[int64(i)]

		// Create a map to match pre and post balances by mint
		tokenBalances := make(map[string]*TokenBalance) // mint -> balance

		// Add pre token balances
		for _, preBalance := range preTokens {
			preAmount, err := decimal.NewFromString(preBalance.UiTokenAmount.UiAmountString)
			if err != nil {
				return TransactionDetail{}, fmt.Errorf("failed to parse pre amount: %v", err)
			}
			tokenBalances[preBalance.Mint] = &TokenBalance{
				Mint:           preBalance.Mint,
				Owner:          preBalance.Owner,
				TokenProgramId: preBalance.ProgramId,
				Decimals:       preBalance.UiTokenAmount.Decimals,
				PreAmount:      preAmount,
			}
		}

		// Add post token balances
		for _, postBalance := range postTokens {
			postAmount, err := decimal.NewFromString(postBalance.UiTokenAmount.UiAmountString)
			if err != nil {
				return TransactionDetail{}, fmt.Errorf("failed to parse post amount: %v", err)
			}
			if bal, exists := tokenBalances[postBalance.Mint]; exists {
				bal.PostAmount = postAmount
				bal.ChangeAmount = postAmount.Sub(bal.PreAmount)
			} else {
				tokenBalances[postBalance.Mint] = &TokenBalance{
					Mint:           postBalance.Mint,
					Owner:          postBalance.Owner,
					TokenProgramId: postBalance.ProgramId,
					Decimals:       postBalance.UiTokenAmount.Decimals,
					PostAmount:     postAmount,
					ChangeAmount:   postAmount,
				}
			}
		}

		// Convert map to array
		tokenBalanceArray := make([]TokenBalance, 0, len(tokenBalances))
		for _, balance := range tokenBalances {
			tokenBalanceArray = append(tokenBalanceArray, *balance)
		}

		accounts[i] = Account{
			Pubkey:         account.Pubkey,
			Signer:         account.Signer,
			Writable:       account.Writable,
			PreSolBalance:  tx.Meta.PreBalances[i],
			PostSolBalance: tx.Meta.PostBalances[i],
			SolChange:      int64(tx.Meta.PostBalances[i]) - int64(tx.Meta.PreBalances[i]),
			TokenBalances:  tokenBalanceArray,
		}
	}

	// Transform instructions with nested inner instructions
	innerInstructionsMap := make(map[uint64][]solana.Instruction)
	for _, innerInst := range tx.Meta.InnerInstructions {
		innerInstructionsMap[innerInst.Index] = innerInst.Instructions
	}

	instructions := make([]InstructionDetail, len(tx.Transaction.Message.Instructions))
	for i, inst := range tx.Transaction.Message.Instructions {
		stackHeight := uint64(0)
		if inst.StackHeight != nil {
			stackHeight = *inst.StackHeight
		}

		instruction := InstructionDetail{
			Accounts:    inst.Accounts,
			Data:        inst.Data,
			ProgramId:   inst.ProgramId,
			Program:     inst.Program,
			StackHeight: stackHeight,
		}

		if inst.Parsed != nil {
			instruction.ParsedType = inst.Parsed.Type
			instruction.ParsedAmount = inst.Parsed.Info.Amount
			instruction.ParsedAuthority = inst.Parsed.Info.Authority
			instruction.ParsedDestination = inst.Parsed.Info.Destination
			instruction.ParsedSource = inst.Parsed.Info.Source
			instruction.ParsedMint = inst.Parsed.Info.Mint

			if inst.Parsed.Info.TokenAmount != nil {
				instruction.ParsedTokenAmountAmount = inst.Parsed.Info.TokenAmount.Amount
				instruction.ParsedTokenAmountDecimals = inst.Parsed.Info.TokenAmount.Decimals
				instruction.ParsedTokenAmountUiAmountString = inst.Parsed.Info.TokenAmount.UiAmountString
			}
		}

		// Process inner instructions
		if inners, exists := innerInstructionsMap[uint64(i)]; exists {
			innerInstructions := make([]InstructionDetail, len(inners))
			for j, inner := range inners {
				innerStackHeight := uint64(0)
				if inner.StackHeight != nil {
					innerStackHeight = *inner.StackHeight
				}

				innerInstruction := InstructionDetail{
					Accounts:    inner.Accounts,
					Data:        inner.Data,
					ProgramId:   inner.ProgramId,
					Program:     inner.Program,
					StackHeight: innerStackHeight,
				}

				if inner.Parsed != nil {
					innerInstruction.ParsedType = inner.Parsed.Type
					innerInstruction.ParsedAmount = inner.Parsed.Info.Amount
					innerInstruction.ParsedAuthority = inner.Parsed.Info.Authority
					innerInstruction.ParsedDestination = inner.Parsed.Info.Destination
					innerInstruction.ParsedSource = inner.Parsed.Info.Source
					innerInstruction.ParsedMint = inner.Parsed.Info.Mint

					if inner.Parsed.Info.TokenAmount != nil {
						innerInstruction.ParsedTokenAmountAmount = inner.Parsed.Info.TokenAmount.Amount
						innerInstruction.ParsedTokenAmountDecimals = inner.Parsed.Info.TokenAmount.Decimals
						innerInstruction.ParsedTokenAmountUiAmountString = inner.Parsed.Info.TokenAmount.UiAmountString
					}
				}

				innerInstructions[j] = innerInstruction
			}
			instruction.InnerInstructions = innerInstructions
		}

		instructions[i] = instruction
	}

	return TransactionDetail{
		BlockTime:            tx.BlockTime,
		Fee:                  tx.Meta.Fee,
		ComputeUnitsConsumed: tx.Meta.ComputeUnitsConsumed,
		Err:                  fmt.Sprintf("%v", tx.Meta.Err),
		LogMessages:          tx.Meta.LogMessages,
		Accounts:             accounts,
		Instructions:         instructions,
		Version:              uint64(tx.Version),
	}, nil
}
