package main

import (
	"flag"
	"log"

	"github.com/redis_internals/config"
	"github.com/redis_internals/server"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "host for the dice server")
	flag.IntVar(&config.Port, "port", 7379, "port for dice server")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("rolling dice")
	server.RunSyncTCPServer()
}
