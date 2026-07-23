package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"rox-khata/internal/db"
	"rox-khata/internal/erp"
	"rox-khata/internal/ledger"
)

func main() {
	log.Println("[Init] Starting Rox Khata Ledger Service...")

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("[Info] No .env file found, relying on system environment variables")
	}

	// 1. Load Configurations from Environment Variables
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/rox_khata?sslmode=disable")

	// 2. Setup Database Connection Pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.InitPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("[Error] Failed to initialize database: %v", err)
	}
	defer pool.Close()
	log.Println("[Init] Database connection pool established successfully.")

	// 3. Initialize Clean Architecture Layers
	repo := ledger.NewLedgerRepository(pool)
	service := ledger.NewLedgerService(repo)
	handler := ledger.NewLedgerHandler(service)

	// ERP Module
	erpRepo := erp.NewERPRepository(pool)
	erpService := erp.NewERPService(erpRepo)
	erpHandler := erp.NewERPHandler(erpService)

	// 4. Set Gin Mode and Initialize Router
	ginMode := getEnv("GIN_MODE", "release")
	gin.SetMode(ginMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Register routes
	handler.RegisterRoutes(router)
	erpHandler.RegisterRoutes(router)

	// 5. Build HTTP Server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// 6. Graceful Shutdown Setup
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[Server] Listening on http://localhost:%s in %s mode\n", port, ginMode)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[Error] Server failed: %v\n", err)
		}
	}()

	// Block until a signal is received
	sig := <-shutdownChan
	log.Printf("[Shutdown] Signal %v received. Shutting down gracefully...\n", sig)

	// Context with timeout to give in-flight requests time to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP Server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Shutdown] Server shutdown error: %v\n", err)
	} else {
		log.Println("[Shutdown] HTTP server stopped cleanly.")
	}

	// Close DB connection pool
	log.Println("[Shutdown] Closing database connection pool...")
	pool.Close()
	log.Println("[Shutdown] Rox Khata Ledger Service stopped.")
}

// getEnv retrieves environment variables or falls back to a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
