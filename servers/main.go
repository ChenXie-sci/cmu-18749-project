package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	// Initialize the server.

	server := NewServer()
	name := "S1"
	ip := "localhost"
	checkpointFreq := 0
	if server == nil {
		fmt.Println("New() returned a nil server. Exiting...")
		return
	}
	arg_len := len(os.Args)
	if arg_len != 4 {
		fmt.Println("should use two arguments.")
		fmt.Println("go run . server_name ip checkpoint_frequency to indicate this is the primary replica")
		return
	}

	name = os.Args[1]
	ip = os.Args[2]
	checkpointFreq, _ = strconv.Atoi(os.Args[3])

	if name != "S1" && name != "S2" && name != "S3" {
		fmt.Println("the server name should be one of S1, S2, S3")
		return
	}

	// Start the server and continue listening for client connections in the background.
	if err := server.Start(name, ip, checkpointFreq); err != nil {
		fmt.Printf("erver could not be started: %s\n", err)
		return
	}

	fmt.Println("Started server waiting for RM's message")

	// time.Sleep(time.Second * 10)
	// server.Close()
	// fmt.Println("Closed server")
	// Block forever.
	select {}
}
