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
    conn.CloseChannel <- true
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

func TestConnectionShouldCloseOnError(t *testing.T) {
    socket := MockConnErrorOnWrite{
        WrittenBytes: new(bytes.Buffer),
        CloseChannel: make(chan bool),
    }

    apn := NewAPNSConnection(socket)

    apn.SendChannel <- &Payload {
        AlertText: "Testing",
        Token: "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f",
    }

    fmt.Printf("Socket had the following bytes written to it:\n%v\n", socket.WrittenBytes)

    connectionClose := <- apn.CloseChannel

    if connectionClose.Error.ErrorCode != 10 {
        t.FailNow()
    }
}

type MockConnErrorOnToken struct {
    WrittenBytes    *bytes.Buffer
    CloseChannel    chan bool
}
func (conn MockConnErrorOnToken) Read(b []byte) (n int, err error) {
    <- conn.CloseChannel
    b[0] = uint8(8) //command
    b[1] = uint8(8) //invalid token
    binary.PutVarint(b[2:], 2)
    return 6, nil
}
func (conn MockConnErrorOnToken) Write(b []byte) (n int, err error) {
    conn.WrittenBytes.Write(b)
    frameLength := binary.BigEndian.Uint32(b[1:5])
    fmt.Printf("Recieved frame length %v\n", frameLength)

    //itemDataSlice := b[5:]

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
        CloseChannel: make(chan bool),
    }

    apn := NewAPNSConnection(socket)

    apn.SendChannel <- &Payload {
        AlertText: "Testing",
        Token: "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f",
    }

    fmt.Printf("Socket had the following bytes written to it:\n%v\n", socket.WrittenBytes)

    connectionClose := <- apn.CloseChannel

    if connectionClose.Error.ErrorCode != 10 {
        t.FailNow()
    }
}