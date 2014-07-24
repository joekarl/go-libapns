go-libapns
==========

APNS library for go

The idea here is to be a simple low level library that will handle establishing a connection and sending push notifications via Apple's apns service with thought towards throughput and performance.

Handles the latest Apple push notification guidelines at https://developer.apple.com/library/ios/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/ApplePushService.html

Specifically will implement the binary framed format by batching push notifications. Each batch will be flushed either every 10ms or when a frame is full. A frame is full when the framed format cannot fit anymore data into a tcp packet (65535 bytes).

##Installation

```bash
> go get github.com/joekarl/go-libapns
```

##Basic Usage
```go
package main

import (
    apns "github.com/joekarl/go-libapns"
)

func main() {
    //tlsConn is a socket connection to apple's gateway
    apnsConnection := apns.NewAPNSConnection(tlsConn)

    payload := &apns.Payload {
        Token: "2ed202ac08ea9...cf8d55910df290567037dcc4",
        AlertText: "This is a push notification!",
    }

    apnsConnection.SendChannel <- payload
    apnsConnection.Disconnect()
}
```
**Note** This example doesn't take into account essential error handling or socket creation details. See below for error handling details

##Error Handling
As per Apple's guidelines, when a connection is closed due to error, the id of the message which caused the error will be transmitted back over the connection. In this case, multiple push notifications may have followed the bad message. These push notifications will be supplied on a channel **as well as any other unsent messages** and will be then available to re-process. Also when writing to the send channel, you should wrap the send with a select and case both the send and connection close channels. This will allow you to correctly handle the async nature of Apple's error handling scheme.

```go
package main

import (
    apns "github.com"
    "crypto/tls"
    "net"
)

func main() {

    apnsConnection := apns.NewAPNSConnection(tlsConn)

    var payload *apns.Payload
    var sendError *apns.ConnectionClose
    for i := 0; i < 1000; i++ {
        if sendError != nil {
            break
        }
        payload = &apns.Payload {
            Token: getTokenForUser(i),
            AlertText: "This is a push notification",
        }

        select {
            case apnsConnection.SendChannel <- payload:
                //hooray! we wrote the payload to the socket
                break
            case sendError = <- apnsConnection.CloseChannel:
                //something happened to our apns connection :(
                //also it has disconnected itself from the socket
                break
        }
    }

    if sendError != nil {
        //*list.List list of payloads that need to be resent
        sendError.UnsentPayloads

        //*apns.Payload payload which apple indicates caused error
        //    (only set if a payload caused the error)
        sendError.ErrorPayload

        //*apns.AppleError actual apple error information
        sendError.Error

        //bool if this is true, then we overflowed our buffer and
        //    some notifications were lost due to error
        sendError.UnsentPayloadBufferOverflow
    }

}
```

##Persistent Connection
go-libapns will use a persistant tcp connection (supplied by the user) to connect to Apple's APNS gateway. This allows for the greatest throughput to Apple's servers. On close or error, this connection will be killed and all unsent push notifications will be supplied for re-process. **Note** Unlike most other APNS libraries, go-libapns will NOT attempt to re-transmit your unsent payloads. Because it is trivial to write this retry logic, go-libapns leaves that to the user to implement as not everyone needs or wants this behavior.

##Push Notification Length
Apple places a strict limit on push notification length (currently at 256 bytes). go-libapns will attempt to fit your push notification into that size limit by first applying all of your supplied custom fields and applying as much of your alert text as possible. This truncation is not without cost as it takes almost twice the time to fix a message that is too long. So if possible, try to find a sweet spot that won't cause truncation to occur. If unable to truncate the message, go-libapns will close it's connection to the APNS gateway (you've been warned).

##Feedback Service
Right now there is no implementation of the feedback service in this library, but one is planned.

##What's with using channels for writing to the connection?
Basically, this makes it easier to synchronize error handling and socket errors. Not sure if this is the best idea, but definitely works.
