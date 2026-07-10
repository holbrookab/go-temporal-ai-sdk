package updates

import (
	"errors"
	"fmt"
)

var ErrStreamNotFound = errors.New("stream not found")

type StreamNotFoundError struct{ StreamID string }

func NewStreamNotFoundError(streamID string) error { return &StreamNotFoundError{StreamID: streamID} }
func (e *StreamNotFoundError) Error() string {
	if e == nil || e.StreamID == "" {
		return ErrStreamNotFound.Error()
	}
	return fmt.Sprintf("stream %q not found", e.StreamID)
}
func (e *StreamNotFoundError) Is(target error) bool { return target == ErrStreamNotFound }
func IsStreamNotFound(err error) bool               { return errors.Is(err, ErrStreamNotFound) }
