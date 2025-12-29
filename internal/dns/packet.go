package dns

type Packet struct {
	Header      Header
	Questions   []Question
	Answers     []Record
	Authorities []Record
	Additionals []Record
}

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
	for i := 0; i < int(h.QDCount); i++ {
		q, err := ParseQuestion(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Questions = append(p.Questions, q)
	}
	p.Answers = make([]Record, 0, limitCount(h.ANCount, MaxRRPerSection))
	for i := 0; i < int(h.ANCount); i++ {
		rr, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Answers = append(p.Answers, rr)
	}
	p.Authorities = make([]Record, 0, limitCount(h.NSCount, MaxRRPerSection))
	for i := 0; i < int(h.NSCount); i++ {
		rr, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Authorities = append(p.Authorities, rr)
	}
	p.Additionals = make([]Record, 0, limitCount(h.ARCount, MaxRRPerSection))
	for i := 0; i < int(h.ARCount); i++ {
		rr, err := ParseRecord(msg, &off)
		if err != nil {
			return Packet{}, err
		}
		p.Additionals = append(p.Additionals, rr)
	}
	return p, nil
}
