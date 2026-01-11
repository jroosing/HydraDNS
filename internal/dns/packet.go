package dns

import "github.com/jroosing/hydradns/internal/helpers"

// Packet represents a complete DNS message (RFC 1035 Section 4.1).
//
// DNS messages are composed of five sections:
//   - Header: Transaction ID, flags, section counts
//   - Questions: What is being asked (1+ questions per message)
//   - Answers: Resource records answering the question
//   - Authorities: Name servers authoritative for the domain
//   - Additionals: Extra records for optimization (e.g., A records for NS)
//
// Uses Record interface for type-safe handling of different record types (A, AAAA, CNAME, etc.).
type Packet struct {
	Header      Header
	Questions   []Question
	Answers     []Record
	Authorities []Record
	Additionals []Record
}

// Marshal serializes the packet to DNS wire format (big-endian).
func (p Packet) Marshal() ([]byte, error) {
	h := Header{
		ID:      p.Header.ID,
		Flags:   p.Header.Flags,
		QDCount: helpers.ClampIntToUint16(len(p.Questions)),
		ANCount: helpers.ClampIntToUint16(len(p.Answers)),
		NSCount: helpers.ClampIntToUint16(len(p.Authorities)),
		ARCount: helpers.ClampIntToUint16(len(p.Additionals)),
	}

	hb, err := h.Marshal()
	if err != nil {
		return nil, err
	}
	// Estimate capacity: header(12) + question(~50) + records(~100 each)
	estimatedSize := HeaderSize + len(p.Questions)*50 + (len(p.Answers)+len(p.Authorities)+len(p.Additionals))*100
	out := make([]byte, 0, estimatedSize)
	out = append(out, hb...)

	for _, q := range p.Questions {
		qb, err := q.Marshal()
		if err != nil {
			return nil, err
		}
		out = append(out, qb...)
	}

	// Marshal answers, authorities, and additionals
	if err := appendRecords(&out, p.Answers); err != nil {
		return nil, err
	}
	if err := appendRecords(&out, p.Authorities); err != nil {
		return nil, err
	}
	if err := appendRecords(&out, p.Additionals); err != nil {
		return nil, err
	}

	return out, nil
}

// appendRecords marshals and appends records to the output buffer.
func appendRecords(out *[]byte, records []Record) error {
	for _, r := range records {
		b, err := MarshalRecord(r)
		if err != nil {
			return err
		}
		*out = append(*out, b...)
	}
	return nil
}
func ParsePacket(msg []byte) (Packet, error) {
	off := 0
	h, err := ParseHeader(msg, &off)
	if err != nil {
		return Packet{}, err
	}

	p := Packet{Header: h}

	// Cap initial allocation to avoid DoS with large counts in header
	// but small actual packet size.
	p.Questions = make([]Question, 0, min(int(h.QDCount), MaxQuestions))
	for range h.QDCount {
		q, err := ParseQuestion(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Questions = append(p.Questions, q)
	}
	p.Answers = make([]Record, 0, min(int(h.ANCount), MaxRRPerSection))
	for range h.ANCount {
		r, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Answers = append(p.Answers, r)
	}
	p.Authorities = make([]Record, 0, min(int(h.NSCount), MaxRRPerSection))
	for range h.NSCount {
		r, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Authorities = append(p.Authorities, r)
	}
	p.Additionals = make([]Record, 0, min(int(h.ARCount), MaxRRPerSection))
	for range h.ARCount {
		r, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Additionals = append(p.Additionals, r)
	}
	return p, nil
}
