package hbt

import (
	"flag"
	"fmt"
	"log"
	"net"
)

func main() {
	var host string
	var port string
	flag.StringVar(&host, "host", "127.0.0.1", "主机")
	flag.StringVar(&port, "port", "8888", "端口")
	flag.Parse()
	fmt.Println(host, port)

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go HbtHandler(conn)
	}
}
