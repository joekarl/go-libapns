package apns

import (
    "bytes"
    "encoding/binary"
    "errors"
    "fmt"
    "net"
    "testing"
    "time"
)

/**
 * Tests related to connection write errors
 */
type MockConnErrorOnWrite struct {
    WrittenBytes    *bytes.Buffer
    CloseChannel    chan bool
}
func (conn MockConnErrorOnWrite) Read(b []byte) (n int, err error) {
    <- conn.CloseChannel
    return -1, errors.New("Socket Closed")
}
func (conn MockConnErrorOnWrite) Write(b []byte) (n int, err error) {
    conn.WrittenBytes.Write(b)
    return len(b), errors.New("Socket Closed")
}
func (conn MockConnErrorOnWrite) Close() error {
    defer func(){conn.CloseChannel <- true}()
    return nil
}
func (conn MockConnErrorOnWrite) LocalAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnWrite) RemoteAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnWrite) SetDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnWrite) SetReadDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnWrite) SetWriteDeadline(t time.Time) error {
    return nil
}

func TestConnectionShouldCloseOnWriteError(t *testing.T) {
    socket := MockConnErrorOnWrite{
        WrittenBytes: new(bytes.Buffer),
        CloseChannel: make(chan bool),
    }

    apn := socketAPNSConnection(socket)

    payload := &Payload {
        AlertText: "Testing",
        Token: "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f",
    }

    apn.SendChannel <- payload
    connectionClose := <- apn.CloseChannel

    if connectionClose.Error.ErrorCode != 10 {
        fmt.Printf("Should have received error 10 for closed socket be received %v\n", connectionClose.Error)
        t.FailNow()
    }
}

/**
 * Tests related to connection write errors
 */
type MockConnErrorOnWrite2 struct {
    WrittenBytes    *bytes.Buffer
    CloseChannel    chan bool
}
func (conn MockConnErrorOnWrite2) Read(b []byte) (n int, err error) {
    fmt.Printf("Read %v\n", conn.CloseChannel)
    return -1, errors.New("Socket Closed on Read")
}
func (conn MockConnErrorOnWrite2) Write(b []byte) (n int, err error) {
    <- conn.CloseChannel
    return len(b), errors.New("Socket Closed on Write")
}
func (conn MockConnErrorOnWrite2) Close() error {
    fmt.Printf("conn close called %v\n", conn.CloseChannel)
    defer func(){conn.CloseChannel <- true}()
    return nil
}
func (conn MockConnErrorOnWrite2) LocalAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnWrite2) RemoteAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnWrite2) SetDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnWrite2) SetReadDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnWrite2) SetWriteDeadline(t time.Time) error {
    return nil
}

func TestConnectionShouldCloseOnReadError(t *testing.T) {

    extSendChannel := make(chan *Payload)
    syncChan := make(chan bool)

    go func(){
        socket := MockConnErrorOnWrite2{
            WrittenBytes: new(bytes.Buffer),
            CloseChannel: make(chan bool),
        }

        apn := socketAPNSConnection(socket)

        for {
            select {
                case p := <- extSendChannel:
                    apn.SendChannel <- p
                    break
                case connectionClose := <- apn.CloseChannel:
                    if connectionClose.Error.ErrorCode != 10 {
                        fmt.Printf("Should have received error 10 for closed socket be received %v\n", connectionClose.Error)
                        syncChan <- true
                        t.FailNow()
                    }
                    syncChan <- true
                    return
            }
        }
    }()

    <- syncChan
}

/**
 * Tests related to apple returned errors
 */
type MockConnErrorOnToken struct {
    WrittenBytes    *bytes.Buffer
    CloseChannel    chan uint32
}
func (conn MockConnErrorOnToken) Read(b []byte) (n int, err error) {
    errorId := <- conn.CloseChannel
    b[0] = uint8(8) //command
    b[1] = uint8(8) //invalid token
    //write error id in big endian
    b[2] = byte(errorId >> 24)
    b[3] = byte(errorId >> 16)
    b[4] = byte(errorId >> 8)
    b[5] = byte(errorId)
    return 6, nil
}
func (conn MockConnErrorOnToken) Write(b []byte) (n int, err error) {
    conn.WrittenBytes.Write(b)
    frameDataSize := binary.BigEndian.Uint32(b[1:5])
    fmt.Printf("FrameDataSize %v\n", frameDataSize)
    frameDataStart := uint64(5)
    firstItemSize := binary.BigEndian.Uint16(b[frameDataStart + 1:frameDataStart + 3])
    idStart := frameDataStart + 3 + uint64(firstItemSize) - 4 - 4 - 1
    errorId := binary.BigEndian.Uint32(b[idStart:idStart + 4])
    defer func(){conn.CloseChannel <- errorId}()
    return len(b), nil
}
func (conn MockConnErrorOnToken) Close() error {
    return nil
}
func (conn MockConnErrorOnToken) LocalAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnToken) RemoteAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnToken) SetDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnToken) SetReadDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnToken) SetWriteDeadline(t time.Time) error {
    return nil
}

func TestConnectionShouldCloseOnAppleResponse(t *testing.T) {
    socket := MockConnErrorOnToken{
        WrittenBytes: new(bytes.Buffer),
        CloseChannel: make(chan uint32),
    }

    apn := socketAPNSConnection(socket)

    token := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f"

    payload := &Payload {
        AlertText: "Testing",
        Token: token,
    }

    apn.SendChannel <- payload

    connectionClose := <- apn.CloseChannel

    if connectionClose.Error.ErrorCode != 8 {
        fmt.Printf("Should have received error 8 for closed socket be received %v\n", connectionClose.Error)
        t.FailNow()
    }

    if connectionClose.ErrorPayload == nil ||
        connectionClose.ErrorPayload.Token != token {
        fmt.Printf("Should have returned payload object but received %v\n", connectionClose.ErrorPayload)
        t.FailNow()
    }
}

type MockConnErrorOnToken2 struct {
    WrittenBytes    *bytes.Buffer
    CloseChannel    chan uint32
}
func (conn MockConnErrorOnToken2) Read(b []byte) (n int, err error) {
    errorId := <- conn.CloseChannel
    b[0] = uint8(8) //command
    b[1] = uint8(8) //invalid token
    //write error id in big endian
    b[2] = byte(errorId >> 24)
    b[3] = byte(errorId >> 16)
    b[4] = byte(errorId >> 8)
    b[5] = byte(errorId)
    return 6, nil
}
func (conn MockConnErrorOnToken2) Write(b []byte) (n int, err error) {
    conn.WrittenBytes.Write(b)
    frameDataSize := binary.BigEndian.Uint32(b[1:5])
    fmt.Printf("FrameBytes %v\n", b)
    fmt.Printf("FrameDataSize %v\n", frameDataSize)
    frameDataStart := uint64(5)
    firstItemSize := binary.BigEndian.Uint16(b[frameDataStart + 1:frameDataStart + 3])
    secondItemStart := frameDataStart + 3 + uint64(firstItemSize)
    secondItemSize := binary.BigEndian.Uint16(b[secondItemStart + 1:secondItemStart + 3])
    idStart := secondItemStart + 3 + uint64(secondItemSize) - 4 - 4 - 1
    fmt.Printf("1 start: %v size: %v, 2 start: %v size: %v\n", frameDataStart, firstItemSize, secondItemStart, secondItemSize)
    fmt.Printf("second bytes %v\n", b[secondItemStart:secondItemStart + uint64(secondItemSize) + 3])
    errorId := binary.BigEndian.Uint32(b[idStart:idStart + 4])
    fmt.Printf("errorId %v\n", errorId)
    defer func(){conn.CloseChannel <- errorId}()
    return len(b), nil
}
func (conn MockConnErrorOnToken2) Close() error {
    return nil
}
func (conn MockConnErrorOnToken2) LocalAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnToken2) RemoteAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnToken2) SetDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnToken2) SetReadDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnToken2) SetWriteDeadline(t time.Time) error {
    return nil
}

func TestConnectionShouldCloseAndReturnUnsentOnAppleResponse(t *testing.T) {

    extSendChannel := make(chan *Payload)
    syncChan := make(chan bool)

    token := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f"
    token2 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8e"
    token3 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8d"
    token4 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8c"

    go func(){

        payload := &Payload {
            AlertText: "Testing",
            Token: token,
        }
        payload2 := &Payload {
            AlertText: "Testing2",
            Token: token2,
        }
        payload3 := &Payload {
            AlertText: "Testing3",
            Token: token3,
        }
        payload4 := &Payload {
            AlertText: "Testing4",
            Token: token4,
        }

        extSendChannel <- payload
        extSendChannel <- payload2
        extSendChannel <- payload3
        extSendChannel <- payload4
    }()

    go func() {
        socket := MockConnErrorOnToken2{
            WrittenBytes: new(bytes.Buffer),
            CloseChannel: make(chan uint32),
        }

        apn := socketAPNSConnection(socket)

        for {
            select {
                case p := <- extSendChannel:
                    if p == nil {
                        return
                    }
                    apn.SendChannel <- p
                    break
                case connectionClose := <- apn.CloseChannel:
                    if connectionClose.Error.ErrorCode != 8 {
                        fmt.Printf("Should have received error 8 for closed socket be received %v\n", connectionClose.Error)
                        syncChan <- true
                        t.FailNow()
                    }

                    if connectionClose.ErrorPayload == nil ||
                        connectionClose.ErrorPayload.Token != token2 {
                        fmt.Printf("Should have returned payload object but received %v\n", connectionClose.ErrorPayload)
                        syncChan <- true
                        t.FailNow()
                    }

                    for e := connectionClose.UnsentPayloads.Front(); e != nil; e = e.Next() {
                        fmt.Printf("Unsent payload %v\n", e.Value.(*Payload))
                    }

                    if connectionClose.UnsentPayloads == nil ||
                        connectionClose.UnsentPayloads.Len() != 2 {
                        fmt.Printf("Should have returned 2 unsent payload objects but received %v len %v\n", connectionClose.UnsentPayloads, connectionClose.UnsentPayloads.Len())
                        syncChan <- true
                        t.FailNow()
                    }

                    if connectionClose.UnsentPayloads.Front().Value.(*Payload).Token != token3 &&
                        connectionClose.UnsentPayloads.Back().Value.(*Payload).Token != token4 {
                        fmt.Printf("Expected to receive specific unsent payloads but received %v len %v\n", connectionClose.UnsentPayloads, connectionClose.UnsentPayloads.Len())
                        syncChan <- true
                        t.FailNow()
                    }

                    if connectionClose.UnsentPayloadBufferOverflow {
                        fmt.Printf("Expected to NOT get buffer overflow indication but did\n")
                        syncChan <- true
                        t.FailNow()
                    }

                    syncChan <- true
                    return
            }
        }
    }()

    <- syncChan
}

type MockConnErrorOnToken3 struct {
    WrittenBytes    *bytes.Buffer
    CloseChannel    chan uint32
}
func (conn MockConnErrorOnToken3) Read(b []byte) (n int, err error) {
    errorId := <- conn.CloseChannel
    b[0] = uint8(8) //command
    b[1] = uint8(8) //invalid token
    //write error id in big endian
    b[2] = byte(errorId >> 24)
    b[3] = byte(errorId >> 16)
    b[4] = byte(errorId >> 8)
    b[5] = byte(errorId)
    return 6, nil
}
func (conn MockConnErrorOnToken3) Write(b []byte) (n int, err error) {
    conn.WrittenBytes.Write(b)
    dataSize := binary.BigEndian.Uint16(b[1:3])
    idStart := 3 + uint64(dataSize) - 4 - 4 - 1
    id := binary.BigEndian.Uint32(b[idStart:idStart + 4])
    //after #4 written, say an error happened on id 2
    if id == 4 {
        defer func(){conn.CloseChannel <- 2}()
    }
    return len(b), nil
}
func (conn MockConnErrorOnToken3) Close() error {
    return nil
}
func (conn MockConnErrorOnToken3) LocalAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnToken3) RemoteAddr() net.Addr {
    return nil
}
func (conn MockConnErrorOnToken3) SetDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnToken3) SetReadDeadline(t time.Time) error {
    return nil
}
func (conn MockConnErrorOnToken3) SetWriteDeadline(t time.Time) error {
    return nil
}

func TestConnectionShouldCloseAndReturnUnsentUpToBufferSizeOnAppleResponse(t *testing.T) {

    extSendChannel := make(chan *Payload)
    syncChan := make(chan bool)

    token := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f"
    token2 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8e"
    token3 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8d"
    token4 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8c"

    go func(){

        payload := &Payload {
            AlertText: "Testing",
            Token: token,
        }
        payload2 := &Payload {
            AlertText: "Testing2",
            Token: token2,
        }
        payload3 := &Payload {
            AlertText: "Testing3",
            Token: token3,
        }
        payload4 := &Payload {
            AlertText: "Testing4",
            Token: token4,
        }

        extSendChannel <- payload
        extSendChannel <- payload2
        extSendChannel <- payload3
        extSendChannel <- payload4
    }()

    go func() {
        socket := MockConnErrorOnToken2{
            WrittenBytes: new(bytes.Buffer),
            CloseChannel: make(chan uint32),
        }

        apn := socketAPNSConnectionBufSize(socket, 1)

        for {
            select {
                case p := <- extSendChannel:
                    if p == nil {
                        return
                    }
                    apn.SendChannel <- p
                    break
                case connectionClose := <- apn.CloseChannel:
                    if connectionClose.Error.ErrorCode != 8 {
                        fmt.Printf("Should have received error 8 for closed socket be received %v\n", connectionClose.Error)
                        syncChan <- true
                        t.FailNow()
                    }

                    if connectionClose.ErrorPayload != nil {
                        fmt.Printf("Should have returned payload object but received %v\n", connectionClose.ErrorPayload)
                        syncChan <- true
                        t.FailNow()
                    }

                    for e := connectionClose.UnsentPayloads.Front(); e != nil; e = e.Next() {
                        fmt.Printf("Unsent payload %v\n", e.Value.(*Payload))
                    }

                    if connectionClose.UnsentPayloads == nil ||
                        connectionClose.UnsentPayloads.Len() != 1 {
                        fmt.Printf("Should have returned 1 unsent payload objects but received %v len %v\n", connectionClose.UnsentPayloads, connectionClose.UnsentPayloads.Len())
                        syncChan <- true
                        t.FailNow()
                    }

                    if connectionClose.UnsentPayloads.Front().Value.(*Payload).Token != token4 {
                        fmt.Printf("Expected to receive specific unsent payloads but received %v len %v\n", connectionClose.UnsentPayloads, connectionClose.UnsentPayloads.Len())
                        syncChan <- true
                        t.FailNow()
                    }

                    if !connectionClose.UnsentPayloadBufferOverflow {
                        fmt.Printf("Expected to get buffer overflow indication but didn't\n")
                        syncChan <- true
                        t.FailNow()
                    }

                    syncChan <- true
                    return
            }
        }
    }()

    <- syncChan
}
