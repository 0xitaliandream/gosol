package solana

import (
	"encoding/json"
	"fmt"
)

type TransactionVersion uint64

// TransactionError rappresenta un errore di transazione
type TransactionError struct {
	InstructionIndex int    `json:"instructionIndex"`
	Error            string `json:"error"`
}

func (v *TransactionVersion) UnmarshalJSON(data []byte) error {
	if string(data) == `"legacy"` {
		*v = 1
		return nil
	}

	// Prova a decodificare come numero
	var num uint64
	if err := json.Unmarshal(data, &num); err != nil {
		return fmt.Errorf("invalid transaction version: %s", string(data))
	}
	*v = TransactionVersion(num)
	return nil
}

// MarshalJSON serializza TransactionVersion correttamente
func (v TransactionVersion) MarshalJSON() ([]byte, error) {
	if v == 1 {
		return json.Marshal("legacy") // Se è -1, serializza come "legacy"
	}
	return json.Marshal(uint64(v)) // Altrimenti come numero
}

// TokenBalance rappresenta le informazioni sul bilancio dei token
type TokenBalance struct {
	AccountIndex  int64         `json:"accountIndex"`
	Mint          string        `json:"mint"`
	Owner         string        `json:"owner"`
	ProgramId     string        `json:"programId"`
	UiTokenAmount UiTokenAmount `json:"uiTokenAmount"`
}

// UiTokenAmount rappresenta l'importo del token con le sue varie rappresentazioni
type UiTokenAmount struct {
	Amount         string   `json:"amount"`
	Decimals       int64    `json:"decimals"`
	UiAmount       *float64 `json:"uiAmount,omitempty"` // Può essere null, quindi usiamo un puntatore
	UiAmountString string   `json:"uiAmountString"`
}

// InnerInstruction rappresenta un'istruzione interna
type InnerInstruction struct {
	Index        uint64        `json:"index"`
	Instructions []Instruction `json:"instructions"`
}

// Instruction rappresenta un'istruzione nella transazione
type Instruction struct {
	Accounts  []string `json:"accounts"`
	Data      string   `json:"data"`
	ProgramId string   `json:"programId"`
	Program   string   `json:"program,omitempty"`
	Parsed    *struct {
		Info ParsedInfo `json:"info"`
		Type string     `json:"type"`
	} `json:"parsed,omitempty"`
	StackHeight *uint64 `json:"stackHeight,omitempty"`
}

// ParsedInfo rappresenta le informazioni analizzate in un'istruzione
type ParsedInfo struct {
	Amount         string         `json:"amount,omitempty"`
	Authority      string         `json:"authority,omitempty"`
	Destination    string         `json:"destination,omitempty"`
	Source         string         `json:"source,omitempty"`
	Mint           string         `json:"mint,omitempty"`
	TokenAmount    *UiTokenAmount `json:"tokenAmount,omitempty"`
	NonceAccount   string         `json:"nonceAccount,omitempty"`
	NonceAuthority string         `json:"nonceAuthority,omitempty"`
}

// Message rappresenta la parte del messaggio della transazione
type Message struct {
	AccountKeys         []Account     `json:"accountKeys"`
	Instructions        []Instruction `json:"instructions"`
	RecentBlockhash     string        `json:"recentBlockhash"`
	AddressTableLookups []struct {
		AccountKey      string  `json:"accountKey"`
		ReadonlyIndexes []int64 `json:"readonlyIndexes"`
		WritableIndexes []int64 `json:"writableIndexes"`
	} `json:"addressTableLookups,omitempty"`
}

// Account rappresenta un account nella transazione
type Account struct {
	Pubkey   string `json:"pubkey"`
	Signer   bool   `json:"signer"`
	Source   string `json:"source"`
	Writable bool   `json:"writable"`
}

// TransactionData rappresenta il campo della transazione
type TransactionData struct {
	Message    Message  `json:"message"`
	Signatures []string `json:"signatures"`
}

// TransactionMeta rappresenta il campo meta nella transazione
type TransactionMeta struct {
	Err                  interface{}        `json:"err"`
	Fee                  uint64             `json:"fee"`
	PreBalances          []uint64           `json:"preBalances"`
	PostBalances         []uint64           `json:"postBalances"`
	InnerInstructions    []InnerInstruction `json:"innerInstructions,omitempty"`
	PreTokenBalances     []TokenBalance     `json:"preTokenBalances,omitempty"`
	PostTokenBalances    []TokenBalance     `json:"postTokenBalances,omitempty"`
	LogMessages          []string           `json:"logMessages,omitempty"`
	ComputeUnitsConsumed uint64             `json:"computeUnitsConsumed"`
}

// SolanaTransaction rappresenta la risposta completa della transazione
type SolanaTransaction struct {
	Slot        uint64             `json:"slot"`
	BlockTime   int64              `json:"blockTime"`
	Transaction TransactionData    `json:"transaction"`
	Meta        *TransactionMeta   `json:"meta,omitempty"`
	Version     TransactionVersion `json:"version"`
}
