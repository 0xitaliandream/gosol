package handlers

import (
	"gosol/internal/db/mysql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	db *mysql.Database
}

func NewWalletHandler(database *mysql.Database) *WalletHandler {
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

	wallet := mysql.Wallet{
		Address: input.Address,
	}

	// Usa direttamente GORM per l'insert
	if err := h.db.GetDB().Create(&wallet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      wallet.ID,
		"address": wallet.Address,
	})
}

func (h *WalletHandler) List(c *gin.Context) {
	var wallets []mysql.Wallet

	if err := h.db.GetDB().Find(&wallets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, wallets)
}

func (h *WalletHandler) Get(c *gin.Context) {
	address := c.Param("address")
	var wallet mysql.Wallet

	if err := h.db.GetDB().Where("address = ?", address).First(&wallet).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}

	c.JSON(http.StatusOK, wallet)
}

func (h *WalletHandler) Delete(c *gin.Context) {
	address := c.Param("address")

	result := h.db.GetDB().Where("address = ?", address).Delete(&mysql.Wallet{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Wallet deleted successfully"})
}
