package main

import (
	"fmt"
	"os"
	"strconv"
)

const (
	name = "GFD"
)

func main() {
	// Initialize the server.
	server := NewServer()
	if server == nil {
		fmt.Println("New() returned a nil server. Exiting...")
		return
	}
	heartbeat_freq := 5
	ip := "localhost"
	if len(os.Args) == 2 {
		ip = os.Args[1]
	} else if len(os.Args) >= 2 {
		ip = os.Args[1]
		heartbeat_freq, _ = strconv.Atoi(os.Args[2])
	}

	// Start the server and continue listening for client connections in the background.
	if err := server.Start(name, heartbeat_freq, ip); err != nil {
		fmt.Printf("KeyValueServer could not be started: %s\n", err)
		return
	}

	// fmt.Println("Started GFD server")

	// time.Sleep(time.Second * 10)
	// server.Close()
	// fmt.Println("Closed server")
	// Block forever.
	select {}
}
