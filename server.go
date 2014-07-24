package main

import (
    "fmt"
    "net"
)

func main() {
    ln, err := net.Listen("tcp", ":8080")
    if err != nil {
    	// handle error
    }
    for {
    	conn, err := ln.Accept()
    	if err != nil {
    		// handle error
    		continue
    	}
    	go handleConnection(conn)
    }
}

func handleConnection(conn net.Conn) {
    buf := make([]byte, 65535)
    for {
        _, err := conn.Read(buf)
        if err != nil {
            fmt.Printf("Read Err: %v\n", err)
            return
        }
        //fmt.Printf("%X\n", buf[:n])
    }
}
