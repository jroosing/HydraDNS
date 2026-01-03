package dns

import (
	"encoding/binary"
	"fmt"
)

// Question represents a DNS question section entry (RFC 1035 Section 4.1.2).
//
// Each question specifies what the client is asking for:
//   - Name: The domain name being queried
//   - Type: The record type requested (A, AAAA, MX, etc.)
//   - Class: Usually ClassIN (Internet)
type Question struct {
	Name  string
	Type  uint16
	Class uint16
}

// Marshal serializes the question to DNS wire format.
func (q Question) Marshal() ([]byte, error) {
	name, err := EncodeName(q.Name)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, len(name)+4)
	b = append(b, name...)
	buf := make([]byte, 4)
	binary.BigEndian.PutUint16(buf[0:2], q.Type)
	binary.BigEndian.PutUint16(buf[2:4], q.Class)
	b = append(b, buf...)
	return b, nil
}

// ParseQuestion parses a question from the message at the given offset.
// It advances *off past the parsed question on success.
func ParseQuestion(msg []byte, off *int) (Question, error) {
	name, err := DecodeName(msg, off)
	if err != nil {
		return Question{}, err
	}
	if *off+4 > len(msg) {
		return Question{}, fmt.Errorf("%w: unexpected EOF while reading DNS question", ErrDNSError)
	}
	q := Question{
		Name:  name,
		Type:  binary.BigEndian.Uint16(msg[*off : *off+2]),
		Class: binary.BigEndian.Uint16(msg[*off+2 : *off+4]),
	}
	*off += 4
	return q, nil
}
