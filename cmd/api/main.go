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
	"gosol/internal/db/clickhouse"
	"gosol/internal/db/mysql"

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

	clickhouseDB, err := clickhouse.NewDatabase(clickhouse.Config{
		Hosts:    cfg.CHHosts,
		Database: cfg.CHDb,
		Username: cfg.CHUser,
		Password: cfg.CHPassword,
		Debug:    cfg.CHDebug,
	}, log)
	if err != nil {
		log.Fatalf("Errore nella connessione al database: %v", err)
	}
	defer clickhouseDB.Close()

	mysqlDB, err := mysql.NewDatabase(mysql.Config{
		Host:     cfg.MySQLHost,
		Port:     cfg.MySQLPort,
		Database: cfg.MySQLDb,
		Username: cfg.MySQLUser,
		Password: cfg.MySQLPassword,
		Debug:    cfg.MySQLDebug,
	}, log)

	err = mysqlDB.FirstMigration()
	if err != nil {
		log.Fatalf("Errore durante la migrazione del database: %v", err)
	}

	router := api.NewRouter(clickhouseDB, mysqlDB)

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
