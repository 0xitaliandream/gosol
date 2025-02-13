package api

import (
	"gosol/internal/api/handlers"
	"gosol/internal/db"

	"github.com/gin-gonic/gin"
)

func NewRouter(database *db.Database) *gin.Engine {
	router := gin.Default()

	// Inizializza gli handlers
	walletHandler := handlers.NewWalletHandler(database)
	// transactionHandler := handlers.NewTransactionHandler(database)
	// jobHandler := handlers.NewJobHandler(database, processor)

	// Gruppo di routes per la v1 dell'API
	v1 := router.Group("/api/v1")
	{
		// Wallet routes
		wallet := v1.Group("/wallets")
		{
			wallet.POST("", walletHandler.Create)
			// wallet.GET("", walletHandler.List)
			// wallet.GET("/:address", walletHandler.Get)
			// wallet.DELETE("/:address", walletHandler.Delete)
		}

		// Transaction routes
		// tx := v1.Group("/transactions")
		// {
		// 	tx.GET("", transactionHandler.List)
		// 	tx.GET("/:signature", transactionHandler.Get)
		// 	tx.GET("/wallet/:address", transactionHandler.GetByWallet)
		// }

		// // Job routes
		// job := v1.Group("/jobs")
		// {
		// 	job.GET("", jobHandler.List)
		// 	job.GET("/:id", jobHandler.Get)
		// 	job.POST("/retry/:id", jobHandler.Retry)
		// }
	}

	return router
}
