package rabbitmq

type WalletRabbitMessage struct {
	ID uint `json:"id"`
}

type TransactionRabbitMessage struct {
	ID        string `json:"id"`
	Signature string `json:"signature"`
}
