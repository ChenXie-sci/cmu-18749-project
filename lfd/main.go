package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	// Initialize the server.
	name := "LFD1"
	heartbeat_freq := 5
	server_name := "S1"
	server_port := "localhost:8003"
	gfd_port := "localhost:8000"
	lfd := NewLFD()
	if lfd == nil {
		fmt.Println("New() returned a nil LFD. Exiting...")
		return
	}
	if len(os.Args) > 1 {
		heartbeat_freq, _ = strconv.Atoi(os.Args[1])
		name = os.Args[2]
		server_name = os.Args[3]
		server_port = os.Args[4]
		gfd_port = os.Args[5]
	}

	// Start the server and continue listening for client connections in the background.
	if err := lfd.Start(name, heartbeat_freq, server_name, server_port, gfd_port); err != nil {
		fmt.Printf("LFD could not be started: %s\n", err)
		return
	}

	fmt.Println("Started LFD with heartbeat_freq: ", heartbeat_freq)

	// time.Sleep(time.Second * 10)
	// server.Close()
	// fmt.Println("Closed server")
	// Block forever.
	select {}
}
