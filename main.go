package main

import (
    "apns"
    "fmt"
    "net"
    "runtime"
    "time"
)

func main() {
    runtime.GOMAXPROCS(4)
    runnerNum := 5
    syncChan := make(chan bool, runnerNum)
    for i := 0; i < runnerNum; i++ {
        go runner(i, syncChan)
    }
    for i := 0; i < runnerNum; i++ {
        <- syncChan
    }
}

func runner(id int, syncChan chan bool) {
    p := &apns.Payload{
        AlertText: "Testing this payload with a really long message that should " +
            "cause the payload to be truncated yay and stuff blah blah blah blah blah blah " +
            "and some more text to really make this much bigger and stuff",
        Badge: 2,
        ContentAvailable: 1,
        Sound: "test.aiff",
        LaunchImage: "launch.png",
    }

    socket, err := net.Dial("tcp","ec2-54-191-81-184.us-west-2.compute.amazonaws.com:8080")
    if err != nil {
        fmt.Printf("Err while connecting: %v\n", err)
        return
    }

    apnConn := apns.NewAPNSConnection(socket)

    start := time.Now().UnixNano()

    sendNum := 100000;
    var connErr *apns.ConnectionClose
    cancelled := false
    for i := 0; i < sendNum; i++ {
        if cancelled {
            sendNum = i
            break
        }
        select {
            case apnConn.SendChannel <- p:
                break
            case connErr = <- apnConn.CloseChannel:
                cancelled = true
                break
        }
    }

    if connErr != nil {
        fmt.Printf("Received Error: %v\n", connErr)
    }

    end := time.Now().UnixNano()

    totalSec := float64(end - start) / 1000000000.0

    fmt.Printf("[id: %v] Sent %v in %f seconds\n", id, sendNum, totalSec)
    fmt.Printf("[id: %v] %f msg/s\n", id, (float64(sendNum) / totalSec))
    syncChan <- true
}
