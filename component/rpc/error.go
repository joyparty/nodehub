package rpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
)

// NewError creates a new error with the given code and message.
func NewError(code codes.Code, msg string, details ...protoadapt.MessageV1) error {
	if len(details) == 0 {
		return status.Error(code, msg)
	}

	s, err := status.New(code, msg).WithDetails(details...)
	if err != nil {
		return err
	}
	return s.Err()
}

// ErrorWithDetail adds details to an existing error.
func ErrorWithDetail(err error, details ...protoadapt.MessageV1) error {
	if len(details) == 0 {
		return err
	}

	s, err := status.Convert(err).WithDetails(details...)
	if err != nil {
		return err
	}
	return s.Err()
}
