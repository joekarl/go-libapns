//Package for creating a connection to Apple's APNS gateway and facilitating
//sending push notifications via that gateway
package apns

import (
	"bytes"
	"container/list"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

//Config for creating an APNS Connection
type APNSConfig struct {
	//number of payloads to keep for error purposes, defaults to 10000
	InFlightPayloadBufferSize int
	//number of milliseconds between frame flushes, defaults to 10
	FramingTimeout int
	//max number of bytes allowed in payload, defaults to 2048
	MaxPayloadSize int
	//bytes for cert.pem : required
	CertificateBytes []byte
	//bytes for key.pem : required
	KeyBytes []byte
	//apple gateway, defaults to "gateway.push.apple.com"
	GatewayHost string
	//apple gateway port, defaults to "2195"
	GatewayPort string
	//max number of bytes to frame data to, defaults to TCP_FRAME_MAX
	//generally best to NOT set this and use the default
	MaxOutboundTCPFrameSize int
	//number of seconds to wait for connection before bailing, defaults to no timeout
	SocketTimeout int
	//number of seconds to wait for Tls handshake to complete before bailing, defaults to no timeout
	TlsTimeout int
}

//Object returned on a connection close or connection error
type ConnectionClose struct {
	//Any payload objects that weren't sent after a connection close
	UnsentPayloads *list.List
	//The error details returned from Apple
	Error *AppleError
	//The payload object that caused the error
	ErrorPayload *Payload
	//True if error payload wasn't found indicating some unsent payloads were lost
	UnsentPayloadBufferOverflow bool
}

//Details from Apple regarding a connection close
type AppleError struct {
	//Internal ID of the message that caused the error
	MessageID uint32
	//Error code returned by Apple (see APPLE_PUSH_RESPONSES)
	ErrorCode uint8
	//String name of error code
	ErrorString string
}

//APNS Connection state
type APNSConnection struct {
	//Channel to send payloads on
	SendChannel chan *Payload
	//Channel that connection close is received on
	CloseChannel chan *ConnectionClose
	//raw socket connection
	socket net.Conn
	//config
	config *APNSConfig
	//Buffer to hold payloads for replay
	inFlightPayloadBuffer *list.List
	//Stateful buffer to hold framed byte data
	inFlightFrameByteBuffer *bytes.Buffer
	//Stateful buffer to hold data while generating item bytes
	inFlightItemByteBuffer *bytes.Buffer
	//Mutex to sync access to Frame byte buffer
	inFlightBufferLock *sync.Mutex
	//Stateful counter to identify payloads for replay
	payloadIdCounter uint32
}

//Wrapper for associating an ID with a Payload object
type idPayload struct {
	//The Payload object
	Payload *Payload
	//The numerical id (from payloadIdCounter) for replay identification
	ID uint32
}

const (
	//Max number of bytes in a TCP frame
	TCP_FRAME_MAX = 65535
	//Number of bytes used in the Apple Notification Header
	//command is 1 byte, frame length is 4 bytes
	NOTIFICATION_HEADER_SIZE = 5
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

//Create a new apns connection with supplied config
//If invalid config an error will be returned
//See APNSConfig object for defaults
func NewAPNSConnection(config *APNSConfig) (*APNSConnection, error) {
	errorStrs := ""

	if config.CertificateBytes == nil || config.KeyBytes == nil {
		errorStrs += "Invalid Key/Certificate bytes\n"
	}
	if config.InFlightPayloadBufferSize < 0 {
		errorStrs += "Invalid InFlightPayloadBufferSize. Should be > 0 (and probably around 10000)\n"
	}
	if config.MaxOutboundTCPFrameSize < 0 || config.MaxOutboundTCPFrameSize > TCP_FRAME_MAX {
		errorStrs += "Invalid MaxOutboundTCPFrameSize. Should be between 0 and TCP_FRAME_MAX (and probably above 2048)\n"
	}
	if config.MaxPayloadSize < 0 {
		errorStrs += "Invalid MaxPayloadSize. Should be greater than 0.\n"
	}

	if errorStrs != "" {
		return nil, errors.New(errorStrs)
	}

	if config.InFlightPayloadBufferSize == 0 {
		config.InFlightPayloadBufferSize = 10000
	}
	if config.MaxOutboundTCPFrameSize == 0 {
		config.MaxOutboundTCPFrameSize = TCP_FRAME_MAX
	}
	if config.FramingTimeout == 0 {
		config.FramingTimeout = 10
	}
	if config.GatewayPort == "" {
		config.GatewayPort = "2195"
	}
	if config.GatewayHost == "" {
		config.GatewayHost = "gateway.push.apple.com"
	}
	if config.MaxPayloadSize == 0 {
		config.MaxPayloadSize = 2048
	}
	if config.TlsTimeout == 0 {
		config.TlsTimeout = 5
	}

	x509Cert, err := tls.X509KeyPair(config.CertificateBytes, config.KeyBytes)
	if err != nil {
		//failed to validate key pair
		return nil, err
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{x509Cert},
		ServerName:   config.GatewayHost,
	}

	tcpSocket, err := net.DialTimeout("tcp",
		config.GatewayHost+":"+config.GatewayPort,
		time.Duration(config.SocketTimeout)*time.Second)
	if err != nil {
		//failed to connect to gateway
		return nil, err
	}

	tlsSocket := tls.Client(tcpSocket, tlsConf)
	tlsSocket.SetDeadline(time.Now().Add(time.Duration(config.TlsTimeout) * time.Second))
	err = tlsSocket.Handshake()
	if err != nil {
		//failed to handshake with tls information
		return nil, err
	}

	//hooray! we're connected
	//reset the deadline so it doesn't fail subsequent writes
	tlsSocket.SetDeadline(time.Time{})

	return socketAPNSConnection(tlsSocket, config), nil
}

//Internal create APNS connection from raw socket
//Starts connection close and send listeners
func socketAPNSConnection(socket net.Conn, config *APNSConfig) *APNSConnection {
	c := new(APNSConnection)
	c.config = config
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

//Disconnect from the Apns Gateway
//Flushes any currently unsent messages before disconnecting from the socket
func (c *APNSConnection) Disconnect() {
	//flush on disconnect
	c.inFlightBufferLock.Lock()
	c.flushBufferToSocket()
	c.inFlightBufferLock.Unlock()
	c.noFlushDisconnect()
}

//internal close socket
func (c *APNSConnection) noFlushDisconnect() {
	c.socket.Close()
}

//go-routine to listen for socket closes or apple response information
func (c *APNSConnection) closeListener(errCloseChannel chan *AppleError) {
	buffer := make([]byte, 6, 6)
	_, err := c.socket.Read(buffer)
	if err != nil {
		errCloseChannel <- &AppleError{
			ErrorCode:   10,
			ErrorString: err.Error(),
			MessageID:   0,
		}
	} else {
		messageId := binary.BigEndian.Uint32(buffer[2:])
		errCloseChannel <- &AppleError{
			ErrorString: APPLE_PUSH_RESPONSES[uint8(buffer[1])],
			ErrorCode:   uint8(buffer[1]),
			MessageID:   messageId,
		}
	}
}

//go-routine to listen for Payloads which should be sent
func (c *APNSConnection) sendListener(errCloseChannel chan *AppleError) {
	var appleError *AppleError

	longTimeoutDuration := 5 * time.Minute
	shortTimeoutDuration := time.Duration(c.config.FramingTimeout) * time.Millisecond
	zeroTimeoutDuration := 0 * time.Millisecond
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
			idPayloadObj := &idPayload{
				Payload: sendPayload,
				ID:      c.payloadIdCounter,
			}
			c.payloadIdCounter++
			c.inFlightPayloadBuffer.PushFront(idPayloadObj)
			//check to see if we've overrun our buffer
			//if so, remove one from the buffer
			if c.inFlightPayloadBuffer.Len() > c.config.InFlightPayloadBufferSize {
				c.inFlightPayloadBuffer.Remove(c.inFlightPayloadBuffer.Back())
			}

			c.bufferPayload(idPayloadObj)

			if shortTimeoutDuration > zeroTimeoutDuration {
				//schedule short timeout
				timeoutTimer.Reset(shortTimeoutDuration)
			} else {
				//flush buffer to socket
				c.inFlightBufferLock.Lock()
				c.flushBufferToSocket()
				c.inFlightBufferLock.Unlock()
				timeoutTimer.Reset(longTimeoutDuration)
			}
			break
		case <-timeoutTimer.C:
			//flush buffer to socket
			c.inFlightBufferLock.Lock()
			c.flushBufferToSocket()
			c.inFlightBufferLock.Unlock()
			timeoutTimer.Reset(longTimeoutDuration)
			break
		case appleError = <-errCloseChannel:
			break
		}
	}

	//gather unsent payload objs
	unsentPayloads := list.New()
	var errorPayload *Payload
	if appleError.ErrorCode != 0 {
		for e := c.inFlightPayloadBuffer.Front(); e != nil; e = e.Next() {
			idPayloadObj := e.Value.(*idPayload)
			if idPayloadObj.ID == appleError.MessageID {
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
			Error:                       appleError,
			UnsentPayloads:              unsentPayloads,
			ErrorPayload:                errorPayload,
			UnsentPayloadBufferOverflow: (unsentPayloads.Len() > 0 && errorPayload == nil),
		}

		close(c.CloseChannel)
	}()
}

//Write buffer payload to tcp frame buffer and flush if tcp frame buffer full
//THREADSAFE (with regard to interaction with the frameBuffer using frameBufferLock)
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
	payloadBytes, err := idPayloadObj.Payload.Marshal(c.config.MaxPayloadSize)
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
	binary.Write(c.inFlightItemByteBuffer, binary.BigEndian, idPayloadObj.ID)

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

	//check to see if we should flush inFlightFrameByteBuffer
	if c.inFlightFrameByteBuffer.Len()+c.inFlightItemByteBuffer.Len()+NOTIFICATION_HEADER_SIZE > TCP_FRAME_MAX {
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

//NOT THREADSAFE (need to acquire inFlightBufferLock before calling)
//Write tcp frame buffer to socket and reset when done
//Close on error
func (c *APNSConnection) flushBufferToSocket() {
	//if buffer not created, or zero length, do nothing
	if c.inFlightFrameByteBuffer == nil || c.inFlightFrameByteBuffer.Len() == 0 {
		return
	}

	bufBytes := c.inFlightFrameByteBuffer.Bytes()

	//write to socket
	_, writeErr := c.socket.Write(bufBytes)
	if writeErr != nil {
		fmt.Printf("Error while writing to socket \n%v\n", writeErr)
		defer c.noFlushDisconnect()
	}
	c.inFlightFrameByteBuffer.Reset()
}
