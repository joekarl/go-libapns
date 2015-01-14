package apns

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func writeToken(b []byte, token string) {
	tokenBytes, _ := hex.DecodeString(token)
	for i := 0; i < 32; i++ {
		b[i] = tokenBytes[i]
	}
}

type MockConnTokens struct {
	WriteHeaderState *bool
	CurrentResponse  **FeedbackResponse
	ResponseChannel  chan *FeedbackResponse
	CloseChannel     chan bool
}

func (conn MockConnTokens) Read(b []byte) (n int, err error) {
	if !(*conn.WriteHeaderState) {
		//write token
		writeToken(b, (*conn.CurrentResponse).Token)

		(*conn.WriteHeaderState) = true

		return 32, nil
	} else {
		select {
		case r := <-conn.ResponseChannel:
			(*conn.CurrentResponse) = r

			//write time in big endian
			b[0] = uint8(r.Timestamp >> 24)
			b[1] = uint8(r.Timestamp >> 16)
			b[2] = uint8(r.Timestamp >> 8)
			b[3] = uint8(r.Timestamp)

			//write token size
			b[4] = uint8(0)
			b[5] = uint8(32)

			(*conn.WriteHeaderState) = false

			return 6, nil
		case <-conn.CloseChannel:
			return 0, io.EOF
		}
	}
}
func (conn MockConnTokens) Write(b []byte) (n int, err error) {
	return 0, nil
}
func (conn MockConnTokens) Close() error {
	return nil
}
func (conn MockConnTokens) LocalAddr() net.Addr {
	return nil
}
func (conn MockConnTokens) RemoteAddr() net.Addr {
	return nil
}
func (conn MockConnTokens) SetDeadline(t time.Time) error {
	return nil
}
func (conn MockConnTokens) SetReadDeadline(t time.Time) error {
	return nil
}
func (conn MockConnTokens) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestFeedbackServiceReadShouldReturnTokens(t *testing.T) {
	var writeHeaderState = true
	var feedbackResponse = &FeedbackResponse{}
	socket := MockConnTokens{
		CurrentResponse:  &feedbackResponse,
		WriteHeaderState: &writeHeaderState,
		ResponseChannel:  make(chan *FeedbackResponse),
		CloseChannel:     make(chan bool),
	}

	token := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f"
	token2 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8e"
	token3 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8d"
	token4 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8c"

	timestamp := uint32(837431)

	go func() {
		r1 := &FeedbackResponse{
			Timestamp: timestamp,
			Token:     token,
		}
		r2 := &FeedbackResponse{
			Timestamp: uint32(837432),
			Token:     token2,
		}
		r3 := &FeedbackResponse{
			Timestamp: uint32(837433),
			Token:     token3,
		}
		r4 := &FeedbackResponse{
			Timestamp: uint32(837434),
			Token:     token4,
		}

		socket.ResponseChannel <- r1
		socket.ResponseChannel <- r2
		socket.ResponseChannel <- r3
		socket.ResponseChannel <- r4
		socket.CloseChannel <- true
	}()

	responses, err := readFromFeedbackService(socket)
	if err != nil {
		fmt.Printf("Shouldn't have received an error but got %v\n", err)
		t.FailNow()
	}

	if responses.Len() != 4 {
		fmt.Printf("Should've received 4 tokens\n")
		t.FailNow()
	}

	var res = responses.Front()
	recToken := res.Value.(*FeedbackResponse).Token
	recTime := res.Value.(*FeedbackResponse).Timestamp
	res = res.Next()
	recToken2 := res.Value.(*FeedbackResponse).Token
	res = res.Next()
	recToken3 := res.Value.(*FeedbackResponse).Token
	res = res.Next()
	recToken4 := res.Value.(*FeedbackResponse).Token

	if recToken != token {
		fmt.Printf("Should've received token %v but got %v\n", token, recToken)
		t.FailNow()
	}

	if recToken2 != token2 {
		fmt.Printf("Should've received token2 %v but got %v\n", token2, recToken2)
		t.FailNow()
	}

	if recToken3 != token3 {
		fmt.Printf("Should've received token3 %v but got %v\n", token3, recToken3)
		t.FailNow()
	}

	if recToken4 != token4 {
		fmt.Printf("Should've received token4 %v but got %v\n", token4, recToken4)
		t.FailNow()
	}

	if recTime != timestamp {
		fmt.Printf("Should've received timestamp %v but got %v\n", timestamp, recTime)
		t.FailNow()
	}
}

type MockConnTokensAndErr struct {
	WriteHeaderState *bool
	CurrentResponse  **FeedbackResponse
	ResponseChannel  chan *FeedbackResponse
	CloseChannel     chan bool
}

func (conn MockConnTokensAndErr) Read(b []byte) (n int, err error) {
	if !(*conn.WriteHeaderState) {
		//write token
		writeToken(b, (*conn.CurrentResponse).Token)

		(*conn.WriteHeaderState) = true

		return 32, nil
	} else {
		select {
		case r := <-conn.ResponseChannel:
			(*conn.CurrentResponse) = r

			//write time in big endian
			b[0] = uint8(r.Timestamp >> 24)
			b[1] = uint8(r.Timestamp >> 16)
			b[2] = uint8(r.Timestamp >> 8)
			b[3] = uint8(r.Timestamp)

			//write token size
			b[4] = uint8(0)
			b[5] = uint8(32)

			(*conn.WriteHeaderState) = false

			return 6, nil
		case <-conn.CloseChannel:
			return 0, errors.New("Some random error")
		}
	}
}
func (conn MockConnTokensAndErr) Write(b []byte) (n int, err error) {
	return 0, nil
}
func (conn MockConnTokensAndErr) Close() error {
	return nil
}
func (conn MockConnTokensAndErr) LocalAddr() net.Addr {
	return nil
}
func (conn MockConnTokensAndErr) RemoteAddr() net.Addr {
	return nil
}
func (conn MockConnTokensAndErr) SetDeadline(t time.Time) error {
	return nil
}
func (conn MockConnTokensAndErr) SetReadDeadline(t time.Time) error {
	return nil
}
func (conn MockConnTokensAndErr) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestFeedbackServiceReadShouldReturnTokensAndError(t *testing.T) {
	var writeHeaderState = true
	var feedbackResponse = &FeedbackResponse{}
	socket := MockConnTokensAndErr{
		CurrentResponse:  &feedbackResponse,
		WriteHeaderState: &writeHeaderState,
		ResponseChannel:  make(chan *FeedbackResponse),
		CloseChannel:     make(chan bool),
	}

	token := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8f"
	token2 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8e"
	token3 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8d"
	token4 := "4ec500020d8350072d2417ba566feda10b2b266558371a65ba67fede21393c8c"

	go func() {
		r1 := &FeedbackResponse{
			Timestamp: uint32(837431),
			Token:     token,
		}
		r2 := &FeedbackResponse{
			Timestamp: uint32(837432),
			Token:     token2,
		}
		r3 := &FeedbackResponse{
			Timestamp: uint32(837433),
			Token:     token3,
		}
		r4 := &FeedbackResponse{
			Timestamp: uint32(837434),
			Token:     token4,
		}

		socket.ResponseChannel <- r1
		socket.ResponseChannel <- r2
		socket.ResponseChannel <- r3
		socket.ResponseChannel <- r4
		socket.CloseChannel <- true
	}()

	responses, err := readFromFeedbackService(socket)
	if err == nil {
		fmt.Printf("Should have received an error\n")
		t.FailNow()
	}

	if responses.Len() != 4 {
		fmt.Printf("Should've received 4 tokens\n")
		t.FailNow()
	}

	var res = responses.Front()
	recToken := res.Value.(*FeedbackResponse).Token
	res = res.Next()
	recToken2 := res.Value.(*FeedbackResponse).Token
	res = res.Next()
	recToken3 := res.Value.(*FeedbackResponse).Token
	res = res.Next()
	recToken4 := res.Value.(*FeedbackResponse).Token

	if recToken != token {
		fmt.Printf("Should've received token %v but got %v\n", token, recToken)
		t.FailNow()
	}

	if recToken2 != token2 {
		fmt.Printf("Should've received token2 %v but got %v\n", token2, recToken2)
		t.FailNow()
	}

	if recToken3 != token3 {
		fmt.Printf("Should've received token3 %v but got %v\n", token3, recToken3)
		t.FailNow()
	}

	if recToken4 != token4 {
		fmt.Printf("Should've received token4 %v but got %v\n", token4, recToken4)
		t.FailNow()
	}
}
