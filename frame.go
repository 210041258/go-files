package testutils

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	// DefaultMaxFrameSize is the maximum allowed frame size (16MB).
	DefaultMaxFrameSize = 16 << 20 // 16 MiB
	// lengthFieldSize is the size of the length prefix (4 bytes).
	lengthFieldSize = 4
)

var (
	ErrFrameTooLarge = errors.New("frame size exceeds maximum")
	ErrIncomplete    = errors.New("incomplete frame")
)

// WriteFrame writes a length‑prefixed frame to w.
// The frame format: [4-byte big-endian length] [payload].
// It returns the number of bytes written (including the length prefix) and any error.
func WriteFrame(w io.Writer, payload []byte) (int, error) {
	if len(payload) > DefaultMaxFrameSize {
		return 0, ErrFrameTooLarge
	}

	// Prepend length as 4-byte big-endian.
	header := make([]byte, lengthFieldSize)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))

	n, err := w.Write(header)
	if err != nil {
		return n, err
	}
	m, err := w.Write(payload)
	return n + m, err
}

// ReadFrame reads a complete length‑prefixed frame from r.
// It returns the payload and any error.
// If the frame exceeds MaxFrameSize, ErrFrameTooLarge is returned.
func ReadFrame(r io.Reader) ([]byte, error) {
	// Read length prefix.
	header := make([]byte, lengthFieldSize)
	if _, err := io.ReadFull(r, header); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrIncomplete
		}
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length > DefaultMaxFrameSize {
		return nil, ErrFrameTooLarge
	}

	// Read payload.
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrIncomplete
		}
		return nil, err
	}
	return payload, nil
}

// Scanner provides a convenient way to read frames one by one.
type Scanner struct {
	r      io.Reader
	max    uint32
	buf    []byte
	err    error
	frame  []byte
}

// NewScanner creates a new frame scanner reading from r.
// It uses DefaultMaxFrameSize as the maximum allowed frame size.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		r:   r,
		max: DefaultMaxFrameSize,
	}
}

// SetMaxFrameSize overrides the default maximum frame size.
func (s *Scanner) SetMaxFrameSize(max uint32) {
	s.max = max
}

// Scan advances the scanner to the next frame, which will then be available
// through the Frame method. It returns false when the scan stops, either by
// reaching the end of the input or an error.
func (s *Scanner) Scan() bool {
	if s.err != nil {
		return false
	}

	// Read length prefix.
	if len(s.buf) < lengthFieldSize {
		s.buf = make([]byte, lengthFieldSize)
	}
	_, err := io.ReadFull(s.r, s.buf[:lengthFieldSize])
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			s.err = ErrIncomplete
		} else {
			s.err = err
		}
		return false
	}
	length := binary.BigEndian.Uint32(s.buf[:lengthFieldSize])
	if length > s.max {
		s.err = ErrFrameTooLarge
		return false
	}

	// Read payload.
	payload := make([]byte, length)
	if _, err = io.ReadFull(s.r, payload); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			s.err = ErrIncomplete
		} else {
			s.err = err
		}
		return false
	}
	s.frame = payload
	return true
}

// Frame returns the most recent frame read by a call to Scan.
func (s *Scanner) Frame() []byte {
	return s.frame
}

// Err returns the first non‑EOF error encountered by the Scanner.
func (s *Scanner) Err() error {
	if s.err == ErrIncomplete {
		return nil // incomplete at EOF is not an error for the scanner
	}
	return s.err
}

// FrameCodec combines WriteFrame and ReadFrame on a single read/writer.
type FrameCodec struct {
	rw io.ReadWriter
}

// NewFrameCodec creates a new codec using the given read/writer.
func NewFrameCodec(rw io.ReadWriter) *FrameCodec {
	return &FrameCodec{rw: rw}
}

// Send encodes and writes a frame.
func (c *FrameCodec) Send(payload []byte) error {
	_, err := WriteFrame(c.rw, payload)
	return err
}

// Receive reads and decodes a frame.
func (c *FrameCodec) Receive() ([]byte, error) {
	return ReadFrame(c.rw)
}

// Example usage (commented out):
//
// func handleConnection(conn net.Conn) {
//     defer conn.Close()
//     codec := frame.NewFrameCodec(conn)
//     for {
//         payload, err := codec.Receive()
//         if err != nil {
//             log.Printf("receive error: %v", err)
//             return
//         }
//         log.Printf("received: %s", payload)
//         if err := codec.Send([]byte("ack")); err != nil {
//             log.Printf("send error: %v", err)
//             return
//         }
//     }
// }