package main

import (
	"fmt"
	"os"
)

const (
	name = "RM"
)

func main() {
	// Initialize the server.
	rm := NewRM()
	if rm == nil {
		fmt.Println("New() returned a nil server. Exiting...")
		return
	}
	config := "Active"
	ip := "localhost"
	if len(os.Args) == 2 {
		ip = os.Args[1]
	} else if len(os.Args) >= 2 {
		ip = os.Args[1]
		config = os.Args[2]
	}

	// Start the server and continue listening for client connections in the background.
	if err := rm.Start(name, config, ip); err != nil {
		fmt.Printf("KeyValueServer could not be started: %s\n", err)
		return
	}

	select {}
}
