package main

type Server interface {
	Start(name string, heartbeat int, ip string) error

	Close()
}
