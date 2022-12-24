package main

type LFD interface {
	Start(name string, heartbeat_freq int, server_name string, server_port string, gfd_port string) error

	Close()
}
