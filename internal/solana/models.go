package solana

import (
	"encoding/json"
	"fmt"
)

type TransactionVersion uint64

func (v *TransactionVersion) UnmarshalJSON(data []byte) error {
	if string(data) == `"legacy"` {
		*v = 1
		return nil
	}

	var num uint64
	if err := json.Unmarshal(data, &num); err != nil {
		return fmt.Errorf("invalid transaction version: %s", string(data))
	}
	*v = TransactionVersion(num)
	return nil
}

func (v TransactionVersion) MarshalJSON() ([]byte, error) {
	if v == 1 {
		return json.Marshal("legacy")
	}
	return json.Marshal(uint64(v))
}

type TokenBalance struct {
	AccountIndex  int64         `json:"accountIndex"`
	Mint          string        `json:"mint"`
	Owner         string        `json:"owner"`
	ProgramId     string        `json:"programId"`
	UiTokenAmount UiTokenAmount `json:"uiTokenAmount"`
}

type UiTokenAmount struct {
	Amount         string   `json:"amount"`
	Decimals       int64    `json:"decimals"`
	UiAmount       *float64 `json:"uiAmount,omitempty"`
	UiAmountString string   `json:"uiAmountString"`
}

type InnerInstruction struct {
	Index        uint64        `json:"index"`
	Instructions []Instruction `json:"instructions"`
}

type Instruction struct {
	Accounts    []string        `json:"accounts"`
	Data        string          `json:"data"`
	ProgramId   string          `json:"programId"`
	Program     string          `json:"program,omitempty"`
	Parsed      json.RawMessage `json:"parsed,omitempty"`
	StackHeight *uint64         `json:"stackHeight,omitempty"`
}

type ParsedStructured struct {
	Info ParsedInfo `json:"info"`
	Type string     `json:"type"`
}

func (i *Instruction) GetParsedData() (*ParsedStructured, error) {
	if len(i.Parsed) == 0 {
		return nil, nil
	}

	var str string
	if err := json.Unmarshal(i.Parsed, &str); err == nil {
		return nil, nil
	}

	var parsed ParsedStructured
	if err := json.Unmarshal(i.Parsed, &parsed); err != nil {
		return nil, err
	}

	return &parsed, nil
}

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

type Account struct {
	Pubkey   string `json:"pubkey"`
	Signer   bool   `json:"signer"`
	Source   string `json:"source"`
	Writable bool   `json:"writable"`
}

type TransactionData struct {
	Message    Message  `json:"message"`
	Signatures []string `json:"signatures"`
}

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

type SolanaTransaction struct {
	Slot        uint64             `json:"slot"`
	BlockTime   int64              `json:"blockTime"`
	Transaction TransactionData    `json:"transaction"`
	Meta        *TransactionMeta   `json:"meta,omitempty"`
	Version     TransactionVersion `json:"version"`
}
