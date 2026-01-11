package main

import (
	"os"

	"github.com/bwmarrin/snowflake"
	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var isDebug = os.Getenv(gin.EnvGinMode) == gin.DebugMode
var logg = logger()

func main() {
	if err := godotenv.Load(); err == nil {
		logg.Info("Loaded .env")
	} else {
		logg.Warn("No .env was set")
	}

	// Initialize database
	if err := InitDB(); err != nil {
		logg.Fatal("Failed to initialize database:", err)
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		logg.Fatal(err)
	}

	r := initRouter()
	setupRoutes(r, node)
	if isDebug {
		hostSite(r)
		setupScrapeRoutes(r)
	}
	logg.Fatal(r.Run())
}

func logger() *log.Logger {
	level := log.InfoLevel
	if isDebug {
		level = log.DebugLevel
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		Level: level,
	})
	return logger
}
