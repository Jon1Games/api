package main

import (
	"fmt"
	"os"

	"intranet/api"
	"intranet/database"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		fmt.Fprintf(os.Stderr, "environment variable 'LISTEN_ADDR' is not set\n")
		os.Exit(1)
	}
	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		fmt.Fprintf(os.Stderr, "environment variable 'LISTEN_PORT' is not set\n")
		os.Exit(1)
	}

	fmt.Printf("Connecting to database\n")
	if err := database.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	fmt.Printf("Starting API server at %s:%s...\n", addr, port)
	api.Main(addr, port)
}
