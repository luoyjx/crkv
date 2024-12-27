package protocol

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"
)

// Codec handles message encoding and decoding over a connection
type Codec struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	mu     sync.Mutex
}

// NewCodec creates a new protocol codec
func NewCodec(conn net.Conn) *Codec {
	return &Codec{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

// ReadMessage reads a message from the connection
func (c *Codec) ReadMessage() (*Message, error) {
	msg, err := DecodeMessage(c.reader)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("connection closed")
		}
		return nil, fmt.Errorf("failed to decode message: %v", err)
	}
	return msg, nil
}

// WriteMessage writes a message to the connection
func (c *Codec) WriteMessage(msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}

	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush message: %v", err)
	}

	return nil
}

// Close closes the codec's connection
func (c *Codec) Close() error {
	return c.conn.Close()
}
