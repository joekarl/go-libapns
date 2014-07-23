package apns

import (
    "bytes"
    "container/list"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "net"
    "time"
)

type ConnectionClose struct {
    UnsentPayloads                  *list.List
    Error                           *AppleError
    ErrorPayload                    *Payload
    UnsentPayloadBufferOverflow     bool
}

type AppleError struct {
    MessageId       uint32
    ErrorCode       uint8
    ErrorString     string
}

type APNSConnection struct {
    socket              net.Conn
    SendChannel         chan *Payload
    CloseChannel        chan *ConnectionClose
    //buffered list of sent push notifications
    //oldest payload is last
    sentPayloadBuffer   *list.List
    sentBufferSize      int
    payloadIdCounter    uint32
    inFlightBuffer      *bytes.Buffer
    inFlightId          uint8
}

type idPayload struct {
    Payload         *Payload
    Id              uint32
}

const (
    TCP_FRAME_MAX = 65535
)

// This enumerates the response codes that Apple defines
// for push notification attempts.
var APPLE_PUSH_RESPONSES = map[uint8]string{
    0:   "NO_ERRORS",
    1:   "PROCESSING_ERROR",
    2:   "MISSING_DEVICE_TOKEN",
    3:   "MISSING_TOPIC",
    4:   "MISSING_PAYLOAD",
    5:   "INVALID_TOKEN_SIZE",
    6:   "INVALID_TOPIC_SIZE",
    7:   "INVALID_PAYLOAD_SIZE",
    8:   "INVALID_TOKEN",
    10:  "SHUTDOWN",
    255: "UNKNOWN",
}

func socketAPNSConnection(socket net.Conn) (*APNSConnection) {
    return socketAPNSConnectionBufSize(socket, 10000)
}

func socketAPNSConnectionBufSize(socket net.Conn, bufferSize int) (*APNSConnection) {
    c := new(APNSConnection)
    c.sentBufferSize = bufferSize
    c.sentPayloadBuffer = list.New()
    c.socket = socket
    c.SendChannel = make(chan *Payload)
    c.CloseChannel = make(chan *ConnectionClose)
    c.inFlightBuffer = new(bytes.Buffer)
    c.inFlightId = 0
    errCloseChannel := make(chan *AppleError)

    go c.closeListener(errCloseChannel)
    go c.sendListener(errCloseChannel)

    return c
}

func (c *APNSConnection) Disconnect() {
    c.socket.Close()
}

func (c *APNSConnection) closeListener(errCloseChannel chan *AppleError) {
    buffer := make([]byte, 6, 6)
    _, err := c.socket.Read(buffer)
    fmt.Printf("Close buffer %x\n", buffer)
    if err != nil {
        errCloseChannel <- &AppleError{
            ErrorCode: 10,
            ErrorString: err.Error(),
            MessageId: 0,
        }
    } else {
        messageId := binary.BigEndian.Uint32(buffer[2:])
        errCloseChannel <- &AppleError{
            ErrorString: APPLE_PUSH_RESPONSES[uint8(buffer[1])],
            ErrorCode: uint8(buffer[1]),
            MessageId: messageId,
        }
    }
}

func (c *APNSConnection) sendListener(errCloseChannel chan *AppleError) {
    var appleError *AppleError

    longTimeoutDuration := 5 * time.Minute
    shortTimeoutDuration := 10 * time.Millisecond
    timeoutTimer := time.NewTimer(longTimeoutDuration)

    for {
        if appleError != nil {
            break
        }
        select {
        case sendPayload := <-c.SendChannel:
            if sendPayload == nil {
                //channel was closed
                return
            }
            //do something here...
            fmt.Printf("Adding payload to flush buffer: %v\n", *sendPayload)
            idPayloadObj := &idPayload{
                Payload: sendPayload,
                Id: c.nextPayloadId(),
            }
            c.sentPayloadBuffer.PushFront(idPayloadObj)
            //check to see if we've overrun our buffer
            //if so, remove one from the buffer
            if c.sentPayloadBuffer.Len() > c.sentBufferSize {
                fmt.Printf("Removing %v from buffer because of overflow, buf len %v\n", *c.sentPayloadBuffer.Back().Value.(*idPayload).Payload, c.sentPayloadBuffer.Len())
                c.sentPayloadBuffer.Remove(c.sentPayloadBuffer.Back())
            }

            c.bufferPayload(idPayloadObj)

            //schedule short timeout
            timeoutTimer.Reset(shortTimeoutDuration)
            break
        case <- timeoutTimer.C:
            //try to flush buffer to socket
            c.flushBufferToSocket()
            timeoutTimer.Reset(longTimeoutDuration)
            break
        case appleError = <- errCloseChannel:
            break
        }
    }

    //gather unsent payload objs
    unsentPayloads := list.New()
    var errorPayload *Payload
    for e := c.sentPayloadBuffer.Front(); e != nil; e = e.Next(){
        idPayloadObj := e.Value.(*idPayload)
        if appleError.MessageId != 0 && idPayloadObj.Id == appleError.MessageId {
            //found error payload, keep track of it and remove from send buffer
            errorPayload = idPayloadObj.Payload
            break
        }
        unsentPayloads.PushFront(idPayloadObj.Payload)
    }

    //connection close channel write and close
    go func() {
        c.CloseChannel <- &ConnectionClose{
            Error: appleError,
            UnsentPayloads: unsentPayloads,
            ErrorPayload: errorPayload,
            UnsentPayloadBufferOverflow: (unsentPayloads.Len() > 0 && errorPayload == nil),
        }

        close(c.CloseChannel)
    }()

    fmt.Printf("Finished listening for payloads\n")
}

func (c *APNSConnection) bufferPayload(idPayloadObj *idPayload) {

    //setup buffer for write
    if c.inFlightBuffer.Len() == 0 {
        binary.Write(c.inFlightBuffer, binary.BigEndian, uint8(2))
        binary.Write(c.inFlightBuffer, binary.BigEndian, uint32(0)) //placeholder
    }

    //gen buffer before writing to socket
    itemBuffer := new(bytes.Buffer)
    c.inFlightId++
    token, err := hex.DecodeString(idPayloadObj.Payload.Token)
    if err != nil {
        fmt.Printf("Failed to decode token for payload %v\n", idPayloadObj.Payload)
        c.Disconnect()
        return
    }
    payloadBytes, err := idPayloadObj.Payload.marshalAlertBodyPayload(256)
    if err != nil {
        fmt.Printf("Failed to marshall payload %v : %v\n", idPayloadObj.Payload, err)
        c.Disconnect()
        return
    }

    //length of token + payload + id + expiretime + priority
    dataLength := 32 + len(payloadBytes) + 4 + 4 + 1;

    binary.Write(itemBuffer, binary.BigEndian, uint8(c.inFlightId)) //payload identifier == 1
    binary.Write(itemBuffer, binary.BigEndian, uint16(dataLength))
    binary.Write(itemBuffer, binary.BigEndian, token)
    binary.Write(itemBuffer, binary.BigEndian, payloadBytes)
    binary.Write(itemBuffer, binary.BigEndian, idPayloadObj.Id)
    binary.Write(itemBuffer, binary.BigEndian, idPayloadObj.Payload.ExpirationTime)
    if idPayloadObj.Payload.Priority != 10 && idPayloadObj.Payload.Priority != 5 {
        idPayloadObj.Payload.Priority = 5
    }
    binary.Write(itemBuffer, binary.BigEndian, idPayloadObj.Payload.Priority)

    if c.inFlightBuffer.Len() + itemBuffer.Len() > TCP_FRAME_MAX {
        c.flushBufferToSocket()
        c.inFlightBuffer = new(bytes.Buffer)
        binary.Write(c.inFlightBuffer, binary.BigEndian, uint8(2))
        binary.Write(c.inFlightBuffer, binary.BigEndian, uint32(0))
    }

    itemBuffer.WriteTo(c.inFlightBuffer)
}

func (c *APNSConnection) flushBufferToSocket() {
    //if buffer not created, or zero length, or just has header information written
    //do nothing
    if c.inFlightBuffer == nil || c.inFlightBuffer.Len() == 0 || c.inFlightBuffer.Len() == 5 {
        return
    }

    bufBytes := c.inFlightBuffer.Bytes()

    //fill in frame length
    frameLength := len(bufBytes) - 5 //full length minus header info
    frameLengthBytes := make([]byte, 4)
    binary.BigEndian.PutUint32(frameLengthBytes, uint32(frameLength))
    bufBytes[1] = frameLengthBytes[0]
    bufBytes[2] = frameLengthBytes[1]
    bufBytes[3] = frameLengthBytes[2]
    bufBytes[4] = frameLengthBytes[3]

    //write to socket
    _, writeErr := c.socket.Write(bufBytes)
    if writeErr != nil {
        fmt.Printf("Error while writing to socket \n%v\n", writeErr)
        c.Disconnect()
    }
}

func (c *APNSConnection) nextPayloadId() uint32 {
    c.payloadIdCounter++
    if c.payloadIdCounter == 0 {
        c.payloadIdCounter = 1
    }
    return c.payloadIdCounter
}
