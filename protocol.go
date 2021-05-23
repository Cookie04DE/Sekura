package rubberhose

import "encoding/gob"

type RequestID uint

const (
	AddRequestID RequestID = iota
	DeleteRequestID
)

type Request struct {
	ID   RequestID
	Data interface{}
}

type AddRequest struct {
	DiskPath string
	Password string
}

type AddResponse struct {
	Error      string
	DevicePath string
}

type DeleteRequest struct {
	DiskPath string
	Password string
}

type DeleteResponse struct {
	Error string
}

func RegisterGob() {
	gob.Register(&Request{})
	gob.Register(&AddRequest{})
	gob.Register(&AddResponse{})
	gob.Register(&DeleteRequest{})
	gob.Register(&DeleteResponse{})
}
