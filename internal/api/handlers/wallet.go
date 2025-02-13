package handlers

import (
	"net/http"

	"gosol/internal/db"
	"gosol/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WalletHandler struct {
	db *db.Database
}

func NewWalletHandler(database *db.Database) *WalletHandler {
	return &WalletHandler{
		db: database,
	}
}

func (h *WalletHandler) Create(c *gin.Context) {
	var input struct {
		Address string `json:"address" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newWallet := models.Wallet{
		ID:      uuid.New().String(),
		Address: input.Address,
	}

	err := h.db.InsertWallet(c.Request.Context(), &newWallet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": newWallet.ID})
}

// func (h *WalletHandler) List(c *gin.Context) {
// 	wallets, err := h.db.ListWallets(c.Request.Context())
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, wallets)
// }

// func (h *WalletHandler) Get(c *gin.Context) {
// 	address := c.Param("address")
// 	wallet, err := h.db.GetWallet(c.Request.Context(), address)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, wallet)
// }

// func (h *WalletHandler) Delete(c *gin.Context) {
// 	address := c.Param("address")
// 	err := h.db.DeleteWallet(c.Request.Context(), address)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"message": "Wallet deleted successfully"})
// }
