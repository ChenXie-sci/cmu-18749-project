package main

type RM interface {
	Start(name string, config string, ip string) error

	Close()
}
