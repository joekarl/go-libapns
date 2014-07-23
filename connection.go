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
    UnsentPayloads  *list.List
    Error           *AppleError
    ErrorPayload    *Payload
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
}

type idPayload struct {
    Payload         *Payload
    Id              uint32
}

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

var PAYLOAD_ID uint32 = 0;

func nextPayloadId() uint32 {
    PAYLOAD_ID++
    if PAYLOAD_ID == 0 {
        PAYLOAD_ID = 1
    }
    return PAYLOAD_ID
}

func NewAPNSConnection(socket net.Conn) (*APNSConnection) {
    c := new(APNSConnection)
    c.sentBufferSize = 10000
    c.sentPayloadBuffer = list.New()
    c.socket = socket
    c.SendChannel = make(chan *Payload)
    c.CloseChannel = make(chan *ConnectionClose)
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
    if err != nil {
        errCloseChannel <- &AppleError{
            ErrorCode: 10,
            ErrorString: err.Error(),
            MessageId: 0,
        }
    } else {
        messageId, _ := binary.Varint(buffer[2:])
        errCloseChannel <- &AppleError{
            ErrorString: APPLE_PUSH_RESPONSES[uint8(buffer[1])],
            ErrorCode: uint8(buffer[1]),
            MessageId: uint32(messageId),
        }
    }
}

func (c *APNSConnection) sendListener(errCloseChannel chan *AppleError) {
    var appleError *AppleError

    for {
        if appleError != nil {
            break
        }
        select {
        case sendPayload := <-c.SendChannel:
            //do something here...
            fmt.Printf("Adding payload to flush buffer: %v\n", *sendPayload)
            idPayloadObj := &idPayload{
                Payload: sendPayload,
                Id: nextPayloadId(),
            }
            c.sentPayloadBuffer.PushFront(idPayloadObj)

            //write to socket
            itemBuffer := new(bytes.Buffer)
            token, err := hex.DecodeString(sendPayload.Token)
            if err != nil {
                fmt.Printf("Failed to decode token for payload %v\n", sendPayload)
                c.Disconnect()
                return
            }
            payloadBytes, err := sendPayload.marshalAlertBodyPayload(256)
            if err != nil {
                fmt.Printf("Failed to marshall payload %v : %v\n", sendPayload, err)
                c.Disconnect()
                return
            }

            payloadLength := 32 + len(payloadBytes) + 4 + 4 + 1;

            binary.Write(itemBuffer, binary.BigEndian, uint8(i))
            binary.Write(itemBuffer, binary.BigEndian, uint16(payloadLength))
            binary.Write(itemBuffer, binary.BigEndian, token)
            binary.Write(itemBuffer, binary.BigEndian, payloadBytes)
            binary.Write(itemBuffer, binary.BigEndian, idPayloadObj.Id)
            binary.Write(itemBuffer, binary.BigEndian, sendPayload.ExpirationTime)
            if sendPayload.Priority != 10 && sendPayload.Priority != 5 {
                sendPayload.Priority = 5
            }
            binary.Write(itemBuffer, binary.BigEndian, sendPayload.Priority)

            _, err := c.socket.Write(itemBuffer.Bytes())
            if err != nil {
                fmt.Printf("Error while writing to socket \n%v\n", err)
                c.Disconnect()
            }

            //check to see if we've overrun our buffer
            //if so, remove one from the buffer
            if c.sentPayloadBuffer.Len() > c.sentBufferSize {
                c.sentPayloadBuffer.Remove(c.sentPayloadBuffer.Back())
            }
            break
        case appleError = <- errCloseChannel:
            break
        }
    }

    unsentPayloads := list.New()
    sentPayloadBufferCopy := list.New()
    sentPayloadBufferCopy.PushBackList(c.sentPayloadBuffer)
    var errorPayload *Payload
    for e := c.sentPayloadBuffer.Back(); e != nil; e = e.Prev() {
        idPayloadObj := e.Value.(*idPayload)
        c.sentPayloadBuffer.Remove(e)
        if idPayloadObj.Id == appleError.MessageId {
            errorPayload = idPayloadObj.Payload
            break
        }
    }

    if errorPayload != nil {
        //dump the rest of the payload buffer into the 
        //unset payloads list
        unsentPayloads.PushBackList(c.sentPayloadBuffer)
    } else {
        //we couldn't find the error payload...
        //but we can assume that the rest of payloads 
        //were unsent because in this case we overran our buffer
        //and the error payload doesn't exist anymore
        unsentPayloads.PushBackList(sentPayloadBufferCopy)
    }

    c.CloseChannel <- &ConnectionClose{
        Error: appleError,
        UnsentPayloads: unsentPayloads,
        ErrorPayload: errorPayload,
    }

    close(c.CloseChannel)
}