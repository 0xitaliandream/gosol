package solana

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gosol/internal/db/mysql"
	"net/http"
)

type Client struct {
	rpcURL     string
	httpClient *http.Client
}

type rpcResponse struct {
	JsonRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

func NewClient(rpcURL string) *Client {
	return &Client{
		rpcURL:     rpcURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) GetSignaturesForAddress(address string, until string, before string, limit int) ([]mysql.Transaction, error) {
	// Costruisci i params includendo solo i campi non vuoti
	params := []any{
		address,
		map[string]any{
			"limit": limit,
		},
	}

	// Aggiungi opzionalmente before e until
	if before != "" {
		params[1].(map[string]any)["before"] = before
	}
	if until != "" {
		params[1].(map[string]any)["until"] = until
	}

	response, err := c.rpcCall("getSignaturesForAddress", params)
	if err != nil {
		return nil, err
	}

	var signatures []mysql.Transaction
	if err := json.Unmarshal(response, &signatures); err != nil {
		return nil, fmt.Errorf("error unmarshaling signatures: %w", err)
	}

	return signatures, nil
}

// GetTransaction restituisce direttamente il json.RawMessage
func (c *Client) GetTransaction(signature string) (*SolanaTransaction, error) {
	params := []any{
		signature,
		map[string]any{
			"encoding":                       "jsonParsed",
			"maxSupportedTransactionVersion": 0,
		},
	}

	response, err := c.rpcCall("getTransaction", params)
	if err != nil {
		return nil, err
	}

	var transaction SolanaTransaction
	if err := json.Unmarshal(response, &transaction); err != nil {
		return nil, fmt.Errorf("error unmarshaling transaction: %w", err)
	}

	return &transaction, nil
}

// Metodo helper per fare chiamate RPC
func (c *Client) rpcCall(method string, params []any) (json.RawMessage, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", c.rpcURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %d - %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}
