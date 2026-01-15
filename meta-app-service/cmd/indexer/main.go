package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"meta-app-service/conf"
	"meta-app-service/controller"
	"meta-app-service/database"
	"meta-app-service/service/indexer_service"
	"meta-app-service/service/temp_deploy_service"
)

var ENV string

func init() {
	flag.StringVar(&ENV, "env", "mainnet", "Environment: loc/mainnet/testnet")
}

// @title           Meta App Service Indexer API
// @version         1.0
// @description     Meta App Service Indexer Service API, provides MetaApp query and deploy functionality
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:7333
// @BasePath  /

// @schemes https http

func main() {
	// Initialize all components
	indexerService, srv, cleanup := initAll()
	defer cleanup()

	// Start indexer service (in goroutine)
	go indexerService.Start()
	log.Println("Indexer service started successfully")

	// Start HTTP API service (in goroutine)
	go startServer(srv)
	log.Println("Indexer API service started successfully")

	// Start temp app cleanup service (in goroutine)
	go startTempAppCleanupService()
	log.Println("Temp app cleanup service started successfully")

	// Wait for shutdown signal
	waitForShutdown()

	log.Println("Shutting down indexer service...")

	// Gracefully shutdown HTTP service
	shutdownServer(srv)

	log.Println("Server exited")
}

// initEnv initialize environment
func initEnv() {
	if ENV == "loc" {
		conf.SystemEnvironmentEnum = conf.LocalEnvironmentEnum
	} else if ENV == "mainnet" {
		conf.SystemEnvironmentEnum = conf.MainnetEnvironmentEnum
	} else if ENV == "testnet" {
		conf.SystemEnvironmentEnum = conf.TestnetEnvironmentEnum
	} else if ENV == "example" {
		conf.SystemEnvironmentEnum = conf.ExampleEnvironmentEnum
	}
	fmt.Printf("Environment: %s\n", ENV)
}

// initAll initialize all components
func initAll() (*indexer_service.IndexerService, *http.Server, func()) {
	// Parse command line parameters
	flag.Parse()

	// Set environment
	initEnv()

	// Initialize configuration
	if err := conf.InitConfig(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}
	log.Printf("Configuration loaded: env=%s, net=%s, port=%s", ENV, conf.Cfg.Net, conf.Cfg.Indexer.Port)

	// Initialize database
	if err := initDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	// Create indexer service
	indexerService, err := indexer_service.NewIndexerService()
	if err != nil {
		log.Fatalf("Failed to create indexer service: %v", err)
	}

	// Setup indexer service router (pass indexerService for scanner access)
	router := controller.SetupIndexerRouter(indexerService)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + conf.Cfg.Indexer.Port,
		Handler: router,
	}

	// Return service instance and cleanup function
	cleanup := func() {
		if database.DB != nil {
			database.DB.Close()
		}
	}

	return indexerService, srv, cleanup
}

// initDatabase initialize database based on configuration
func initDatabase() error {
	dbType := database.DBType(conf.Cfg.Database.IndexerType)

	switch dbType {
	case database.DBTypePebble:
		config := &database.PebbleConfig{
			DataDir: conf.Cfg.Database.DataDir,
		}
		return database.InitDatabase(database.DBTypePebble, config)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// startServer start HTTP server
func startServer(srv *http.Server) {
	log.Printf("Indexer API service starting on port %s...", conf.Cfg.Indexer.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// waitForShutdown wait for shutdown signal
func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}

// shutdownServer gracefully shutdown server
func shutdownServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
}

// startTempAppCleanupService 启动临时应用清理服务
// 每小时执行一次清理过期临时应用
func startTempAppCleanupService() {
	cleanupService := temp_deploy_service.NewTempDeployService()

	// 立即执行一次清理
	if err := cleanupService.CleanupExpiredTempApps(); err != nil {
		log.Printf("Failed to cleanup expired temp apps: %v", err)
	}

	// 创建定时器，每小时执行一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := cleanupService.CleanupExpiredTempApps(); err != nil {
			log.Printf("Failed to cleanup expired temp apps: %v", err)
		} else {
			log.Println("Temp app cleanup completed successfully")
		}
	}
}
