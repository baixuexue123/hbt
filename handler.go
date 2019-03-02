package hbt

import (
	"bufio"
	"context"
	"fmt"
	"net"
)

type Handler interface {
	Handle(context.Context, interface{})
}

func HbtHandler(c net.Conn) {
	peer := c.RemoteAddr().String()
	fmt.Println("New connection from ", peer)
	defer c.Close()
	scanner := bufio.NewScanner(c)
	scanner.Split(ScanCRLF)
	for scanner.Scan() {
		msg := scanner.Text()
		if msg == "ping" {
			c.Write(reply)
		} else {
			fmt.Println("Connection: ", peer, "ERROR: ", msg)
			break
		}
	}
	fmt.Println("Connection:", peer, "lost")
}
