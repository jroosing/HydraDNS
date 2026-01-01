package dns

// Packet represents a complete DNS message (RFC 1035 Section 4).
//
// A DNS packet consists of a header and four sections:
//   - Questions: What the client is asking
//   - Answers: Resource records answering the question
//   - Authorities: Nameserver records pointing to authorities
//   - Additionals: Extra records (e.g., glue records, EDNS OPT)
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
		QDCount: uint16(len(p.Questions)),
		ANCount: uint16(len(p.Answers)),
		NSCount: uint16(len(p.Authorities)),
		ARCount: uint16(len(p.Additionals)),
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
	for _, rr := range p.Answers {
		b, err := rr.Marshal()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	for _, rr := range p.Authorities {
		b, err := rr.Marshal()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	for _, rr := range p.Additionals {
		b, err := rr.Marshal()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
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
	limitCount := func(count uint16, limit int) int {
		if int(count) > limit {
			return limit
		}
		return int(count)
	}

	p.Questions = make([]Question, 0, limitCount(h.QDCount, MaxQuestions))
	for range h.QDCount {
		q, err := ParseQuestion(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Questions = append(p.Questions, q)
	}
	p.Answers = make([]Record, 0, limitCount(h.ANCount, MaxRRPerSection))
	for range h.ANCount {
		rr, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Answers = append(p.Answers, rr)
	}
	p.Authorities = make([]Record, 0, limitCount(h.NSCount, MaxRRPerSection))
	for range h.NSCount {
		rr, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Authorities = append(p.Authorities, rr)
	}
	p.Additionals = make([]Record, 0, limitCount(h.ARCount, MaxRRPerSection))
	for range h.ARCount {
		rr, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Additionals = append(p.Additionals, rr)
	}
	return p, nil
}
