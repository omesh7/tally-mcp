package main

import (
	"context"
	"fmt"
	"time"

	"github.com/omesh7/tally-mcp/internal/tally"
)

func main() {
	fmt.Println("=== TallyMCP Date Filter Test Client ===")
	client := tally.NewClient("localhost", 9000, 15*time.Second)
	ctx := context.Background()

	if err := client.Ping(ctx); err != nil {
		fmt.Printf("Error connecting to Tally: %v\n", err)
		return
	}
	fmt.Println("Successfully connected to Tally Prime.")
}
