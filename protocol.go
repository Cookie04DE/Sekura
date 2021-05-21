package rubberhose

import "encoding/gob"

type RequestID uint

const (
	AddRequestID RequestID = iota
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

func RegisterGob() {
	gob.Register(&Request{})
	gob.Register(&AddRequest{})
	gob.Register(&AddResponse{})
}
