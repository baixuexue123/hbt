package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/baixuexue123/hbt"
)

var host string
var port string

func init() {
	flag.StringVar(&host, "host", "127.0.0.1", "host")
	flag.StringVar(&port, "port", "8888", "port")
}

func main() {
	flag.Parse()

	TServer := hbt.NewTCPServer(fmt.Sprintf("%s:%s", host, port))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		TServer.Shutdown()
	}()

	err := TServer.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
