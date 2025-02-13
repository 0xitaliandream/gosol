// cmd/api/main.go

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gosol/config"
	"gosol/internal/api"
	"gosol/internal/db"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.LoadAPIConfig()
	if err != nil {
		log.Fatalf("Errore nel caricamento della configurazione: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.NewDatabase(db.Config{
		Hosts:    cfg.DBHosts,
		Database: cfg.DBName,
		Username: cfg.DBUser,
		Password: cfg.DBPassword,
		Debug:    cfg.DBDebug,
	}, log)
	if err != nil {
		log.Fatalf("Errore nella connessione al database: %v", err)
	}
	defer database.Close()

	router := api.NewRouter(database)

	g, ctx := errgroup.WithContext(ctx)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.API_PORT),
		Handler: router,
	}

	g.Go(func() error {
		log.Infof("Server API avviato sulla porta %s", cfg.API_PORT)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("errore nel server HTTP: %v", err)
		}
		return nil
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-quit:
			log.Infof("Ricevuto segnale di shutdown: %v", sig)
			cancel()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("errore nel graceful shutdown del server: %v", err)
			}

			return nil
		}
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("Errore durante l'esecuzione: %v", err)
	}

	log.Info("Server terminato con successo")
}
