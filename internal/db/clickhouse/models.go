package clickhouse

import "gosol/internal/solana"

type TransactionDetail struct {
	ID                   string                    `json:"id"`
	TransactionID        string                    `json:"transaction_id"`
	Accounts             []solana.Account          `json:"accounts"`
	ComputeUnitsConsumed uint64                    `json:"compute_units_consumed"`
	Fee                  uint64                    `json:"fee"`
	Err                  string                    `json:"err"`
	LogMessages          []string                  `json:"log_messages"`
	PreTokenBalances     []solana.TokenBalance     `json:"pre_token_balances"`
	PostTokenBalances    []solana.TokenBalance     `json:"post_token_balances"`
	PreSolBalances       []uint64                  `json:"pre_sol_balances"`
	PostSolBalances      []uint64                  `json:"post_sol_balances"`
	Instructions         []solana.Instruction      `json:"instructions"`
	InnerInstructions    []solana.InnerInstruction `json:"inner_instructions"`
	Version              solana.TransactionVersion `json:"version"`
}
