package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gosol/internal/db/mysql"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig contiene la configurazione per il sistema di retry
type RetryConfig struct {
	MaxRetries     int           // Numero massimo di tentativi
	InitialBackoff time.Duration // Tempo di attesa iniziale
	MaxBackoff     time.Duration // Tempo massimo di attesa
	BackoffFactor  float64       // Fattore di moltiplicazione per il backoff
	JitterFactor   float64       // Fattore di randomizzazione (0-1)
}

// DefaultRetryConfig fornisce una configurazione di default ragionevole
var DefaultRetryConfig = RetryConfig{
	MaxRetries:     5,
	InitialBackoff: 2 * time.Second,
	MaxBackoff:     45 * time.Second,
	BackoffFactor:  2.0,
	JitterFactor:   0.2,
}

// rpcResponse struttura per le risposte RPC
type rpcResponse struct {
	JsonRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

// RPCError rappresenta un errore RPC custom
type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error: %d - %s", e.Code, e.Message)
}

// Client esteso con configurazione retry
type Client struct {
	rpcURL      string
	httpClient  *http.Client
	retryConfig RetryConfig
}

// NewClient ora accetta anche una configurazione retry opzionale
func NewClient(rpcURL string, retryConfig *RetryConfig) *Client {
	config := DefaultRetryConfig
	if retryConfig != nil {
		config = *retryConfig
	}

	return &Client{
		rpcURL:      rpcURL,
		httpClient:  &http.Client{},
		retryConfig: config,
	}
}

// calculateBackoff calcola il tempo di attesa con jitter
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Calcola il backoff base (exponential)
	backoff := float64(c.retryConfig.InitialBackoff) * math.Pow(c.retryConfig.BackoffFactor, float64(attempt))

	// Applica il max backoff
	if backoff > float64(c.retryConfig.MaxBackoff) {
		backoff = float64(c.retryConfig.MaxBackoff)
	}

	// Applica il jitter
	jitter := (rand.Float64()*2 - 1) * c.retryConfig.JitterFactor * backoff
	backoff = backoff + jitter

	return time.Duration(backoff)
}

// isRetryableError determina se un errore è recuperabile
func isRetryableError(err error, statusCode int) bool {
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// Controlla specifici errori RPC di Solana che sono recuperabili
	if rpcErr, ok := err.(*RPCError); ok {
		retryableCodes := map[int]bool{
			-32000: true, // Generic rate limit/overload response
			-32005: true, // Node is behind
			-32007: true, // Transaction signature verification failure
			-32008: true, // Transaction failed - timeout
			-32009: true, // Transaction failed - block height exceeded
		}
		return retryableCodes[rpcErr.Code]
	}

	return false
}

// rpcCall con retry
func (c *Client) rpcCall(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		// Se non è il primo tentativo, aspetta secondo il backoff
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, statusCode, err := c.doRPCCall(ctx, method, params)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Se l'errore non è recuperabile o siamo all'ultimo tentativo, ritorna l'errore
		if !isRetryableError(err, statusCode) || attempt == c.retryConfig.MaxRetries {
			break
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doRPCCall esegue la singola chiamata RPC
func (c *Client) doRPCCall(ctx context.Context, method string, params []any) (json.RawMessage, int, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("error decoding response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, resp.StatusCode, &RPCError{
			Code:    rpcResp.Error.Code,
			Message: rpcResp.Error.Message,
		}
	}

	// Se il risultato è null o vuoto, trattiamolo come un errore recuperabile
	if len(rpcResp.Result) == 0 || string(rpcResp.Result) == "null" {
		return nil, resp.StatusCode, &RPCError{
			Code:    -32000,
			Message: "empty response from node (possibly rate limited)",
		}
	}

	return rpcResp.Result, resp.StatusCode, nil
}

// GetSignaturesForAddress con context
func (c *Client) GetSignaturesForAddress(ctx context.Context, address string, until string, before string, limit int) ([]mysql.Transaction, error) {
	params := []any{
		address,
		map[string]any{
			"limit": limit,
		},
	}

	if before != "" {
		params[1].(map[string]any)["before"] = before
	}
	if until != "" {
		params[1].(map[string]any)["until"] = until
	}

	response, err := c.rpcCall(ctx, "getSignaturesForAddress", params)
	if err != nil {
		return nil, err
	}

	var signatures []mysql.Transaction
	if err := json.Unmarshal(response, &signatures); err != nil {
		return nil, fmt.Errorf("error unmarshaling signatures: %w", err)
	}

	return signatures, nil
}

// GetTransaction con context
func (c *Client) GetTransaction(ctx context.Context, signature string) (*SolanaTransaction, error) {
	params := []any{
		signature,
		map[string]any{
			"encoding":                       "jsonParsed",
			"maxSupportedTransactionVersion": 0,
		},
	}

	response, err := c.rpcCall(ctx, "getTransaction", params)
	if err != nil {
		return nil, err
	}

	var transaction SolanaTransaction
	if err := json.Unmarshal(response, &transaction); err != nil {
		return nil, fmt.Errorf("error unmarshaling transaction: %w", err)
	}

	return &transaction, nil
}
