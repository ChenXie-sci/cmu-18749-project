package main

type Server interface {
	Start(name string, ip string, checkpointFreq int) error

	Close()
}
