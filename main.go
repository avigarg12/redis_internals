package main

import (
	"flag"
	"log"

	"redis_internals/config"
	"redis_internals/server"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "host for the dice server")
	flag.IntVar(&config.Port, "port", 7379, "port for dice server")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("rolling dice")

	if err := server.RunAsyncTCPServer(); err != nil {
		log.Fatal(err)
	}
}
