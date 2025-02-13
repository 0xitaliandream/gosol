package handlers

// import (
// 	"net/http"

// 	"gosol/internal/db"

// 	"github.com/gin-gonic/gin"
// )

// type TransactionHandler struct {
// 	db *db.Database
// }

// func NewTransactionHandler(database *db.Database) *TransactionHandler {
// 	return &TransactionHandler{
// 		db: database,
// 	}
// }

// func (h *TransactionHandler) List(c *gin.Context) {
// 	limit := c.DefaultQuery("limit", "100")
// 	offset := c.DefaultQuery("offset", "0")

// 	txs, err := h.db.ListTransactions(c.Request.Context(), limit, offset)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, txs)
// }

// func (h *TransactionHandler) Get(c *gin.Context) {
// 	signature := c.Param("signature")
// 	tx, err := h.db.GetTransaction(c.Request.Context(), signature)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, tx)
// }

// func (h *TransactionHandler) GetByWallet(c *gin.Context) {
// 	address := c.Param("address")
// 	limit := c.DefaultQuery("limit", "100")
// 	offset := c.DefaultQuery("offset", "0")

// 	txs, err := h.db.GetWalletTransactions(c.Request.Context(), address, limit, offset)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, txs)
// }
