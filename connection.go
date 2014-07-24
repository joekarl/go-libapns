package apns

import (
    "bytes"
    "container/list"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "net"
    "sync"
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
    inFlightPayloadBuffer           *list.List
    inFlightPayloadBufferSize       int
    inFlightFrameByteBuffer         *bytes.Buffer
    inFlightItemByteBuffer          *bytes.Buffer
    inFlightBufferLock              *sync.Mutex
    payloadIdCounter                uint32
}

type APNSConnectionConfig struct {

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
    128: "INVALID_FRAME_ITEM_ID", //this is not documented, but ran across it in testing
    255: "UNKNOWN",
}

func NewAPNSConnection(socket net.Conn) (*APNSConnection) {
    return socketAPNSConnection(socket)
}

func socketAPNSConnection(socket net.Conn) (*APNSConnection) {
    return socketAPNSConnectionBufSize(socket, 10000)
}

func socketAPNSConnectionBufSize(socket net.Conn, bufferSize int) (*APNSConnection) {
    c := new(APNSConnection)
    c.inFlightPayloadBufferSize = bufferSize
    c.inFlightPayloadBuffer = list.New()
    c.socket = socket
    c.SendChannel = make(chan *Payload)
    c.CloseChannel = make(chan *ConnectionClose)
    c.inFlightFrameByteBuffer = new(bytes.Buffer)
    c.inFlightItemByteBuffer = new(bytes.Buffer)
    c.inFlightBufferLock = new(sync.Mutex)
    c.payloadIdCounter = 0
    errCloseChannel := make(chan *AppleError)

    go c.closeListener(errCloseChannel)
    go c.sendListener(errCloseChannel)

    return c
}

func (c *APNSConnection) Disconnect() {
    //flush on disconnect
    c.inFlightBufferLock.Lock()
    c.flushBufferToSocket()
    c.inFlightBufferLock.Unlock()
    c.noFlushDisconnect()
}

func (c *APNSConnection) noFlushDisconnect() {
    c.socket.Close()
}

func (c *APNSConnection) closeListener(errCloseChannel chan *AppleError) {
    buffer := make([]byte, 6, 6)
    _, err := c.socket.Read(buffer)
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
            //fmt.Printf("Adding payload to flush buffer: %v\n", *sendPayload)
            idPayloadObj := &idPayload{
                Payload: sendPayload,
                Id: c.payloadIdCounter,
            }
            c.payloadIdCounter++
            c.inFlightPayloadBuffer.PushFront(idPayloadObj)
            //check to see if we've overrun our buffer
            //if so, remove one from the buffer
            if c.inFlightPayloadBuffer.Len() > c.inFlightPayloadBufferSize {
                //fmt.Printf("Removing %v from buffer because of overflow, buf len %v\n", *c.inFlightPayloadBuffer.Back().Value.(*idPayload).Payload, c.inFlightPayloadBuffer.Len())
                c.inFlightPayloadBuffer.Remove(c.inFlightPayloadBuffer.Back())
            }

            c.bufferPayload(idPayloadObj)

            //schedule short timeout
            timeoutTimer.Reset(shortTimeoutDuration)
            break
        case <- timeoutTimer.C:
            //flush buffer to socket
            c.inFlightBufferLock.Lock()
            c.flushBufferToSocket()
            c.inFlightBufferLock.Unlock()
            timeoutTimer.Reset(longTimeoutDuration)
            break
        case appleError = <- errCloseChannel:
            break
        }
    }

    //gather unsent payload objs
    unsentPayloads := list.New()
    var errorPayload *Payload
    if appleError.ErrorCode != 0 {
        for e := c.inFlightPayloadBuffer.Front(); e != nil; e = e.Next(){
            idPayloadObj := e.Value.(*idPayload)
            if idPayloadObj.Id == appleError.MessageId {
                //found error payload, keep track of it and remove from send buffer
                errorPayload = idPayloadObj.Payload
                break
            }
            unsentPayloads.PushFront(idPayloadObj.Payload)
        }
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

/**
 * THREADSAFE (with regard to interaction with the frameBuffer using frameBufferLock)
 * Must send in empty itemBuffer and empty frameBuffer
 */
func (c *APNSConnection) bufferPayload(idPayloadObj *idPayload) {
    //acquire lock to tcp buffer to do length checking, buffer writing,
    //and potentially flush buffer
    c.inFlightBufferLock.Lock()

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

    //write token
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint8(1))
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint16(32))
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, token)

    //write payload
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint8(2))
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint16(len(payloadBytes)))
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, payloadBytes)

    //write id
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint8(3))
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint16(4))
    binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, idPayloadObj.Id)

    //write expire date if set
    if idPayloadObj.Payload.ExpirationTime != 0 {
        binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint8(4))
        binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint16(4))
        binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, idPayloadObj.Payload.ExpirationTime)
    }

    //write priority if set correctly
    if idPayloadObj.Payload.Priority == 10 || idPayloadObj.Payload.Priority == 5 {
        binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint8(5))
        binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, uint16(4))
        binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, idPayloadObj.Payload.Priority)
    }

    //check to see if we should flush inFlightTCPBuffer
    if c.inFlightFrameByteBuffer.Len() + c.inFlightItemByteBuffer.Len() > TCP_FRAME_MAX {
        c.flushBufferToSocket()
    }

    //write header info and item info
    binary.Write(c.inFlightFrameByteBuffer, binary.BigEndian, uint8(2))
    binary.Write(c.inFlightFrameByteBuffer, binary.BigEndian, uint32(c.inFlightItemByteBuffer.Len()))
    c.inFlightItemByteBuffer.WriteTo(c.inFlightFrameByteBuffer)

    c.inFlightItemByteBuffer.Reset()

    //unlock byte buffer when finished writing to it
    c.inFlightBufferLock.Unlock()
}

/**
 * NOT THREADSAFE (need to acquire inFlightBufferLock before calling)
 */
func (c *APNSConnection) flushBufferToSocket() {
    //if buffer not created, or zero length, or just has header information written
    //do nothing
    if c.inFlightFrameByteBuffer == nil || c.inFlightFrameByteBuffer.Len() == 0 {
        return
    }

    bufBytes := c.inFlightFrameByteBuffer.Bytes()

    //fmt.Printf("Flushing buffer %x\n", bufBytes)

    //write to socket
    _, writeErr := c.socket.Write(bufBytes)
    if writeErr != nil {
        fmt.Printf("Error while writing to socket \n%v\n", writeErr)
        defer c.noFlushDisconnect()
    }
    c.inFlightFrameByteBuffer.Reset()
}
