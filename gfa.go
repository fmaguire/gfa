// Package gfa is a Go library for working with the Graphical Fragment Assembly (GFA) format.
/*

For more information:
    GFA Format Specification
    https://github.com/GFA-spec/GFA-spec

This package currently only conforms to GFA1 spec.
*/
package gfa

import (
	"bytes"
	"fmt"
)

// The GFA type
type GFA struct {
	Metadata     *header // metadata contains both the header and comments
	segments     []*segment
	links        []*link
	containments []containment       // TODO: not yet implemented
	paths        []path              // TODO: not yet implemented
	segRecord    map[string]struct{} // prevent duplicate segment IDs being added
}

// Returns a new GFA instance
func NewGFA() (*GFA, error) {
	return &GFA{
		Metadata:  NewHeader(),
		segRecord: make(map[string]struct{}),
	}, nil
}

// Returns the GFA segments
func (self *GFA) GetSegments() []*segment {
	return self.segments
}

// Returns the GFA links
func (self *GFA) GetLinks() []*link {
	return self.links
}

// A header contains a type field (required) and a version number field (optional)
type header struct {
	entryType string
	vn        int      // GFA version number
	comments  [][]byte // storing comments with header for simplicity
}

// Returns a new header
func NewHeader() *header {
	return &header{entryType: "H"}
}

// Returns the formatted header line
func (self *header) Header() string {
	return fmt.Sprintf("%v\tVN:Z:%v", self.entryType, self.vn)
}

// Updates the header with a version number
func (self *header) AddVersionNumber(v int) error {
	switch v {
	case 0:
		return fmt.Errorf("GFA instance already has a version number attached")
	case 1:
		self.vn = v
	case 2:
		return fmt.Errorf("GFA version 2 is currently unsupported...")
	default:
		return fmt.Errorf("GFA format must be either version 1 or version 2")
	}
	return nil
}

// Returns the GFA version number
func (self *header) Version() int {
	return self.vn
}

// Adds comments to a GFA header
func (self *header) AddComment(c []byte) {
	comment := append([]byte("#\t"), c...)
	self.comments = append(self.comments, comment)
}

// Returns a string containing the comments from a GFA header
func (self *header) Comments() string {
	return fmt.Sprintf("%s", bytes.Join(self.comments, []byte("\n")))
}

// An interface for the non-comment/header GFA lines
type gfaLine interface {
	PrintGFAline() string
	PrintType() string
	Add(*GFA) error
}

// A segment contains a type field, name and sequence (all required), plus optional fields (length, ...)
type segment struct {
	entryType string
	name      []byte
	sequence  []byte // this is technically not required by the spec but I have set it as required here
	length    int
	readCount int
	fragCount int
	kmerCount int
	checksum  []byte
	uri       string
}

// Segment constructor
func NewSegment(n, seq []byte, optional ...[]byte) (*segment, error) {
	if bytes.ContainsAny(n, "+-*= ") {
		return nil, fmt.Errorf("Segment name can't contain +/-/*/= or whitespace")
	}
	if len(seq) == 0 {
		return nil, fmt.Errorf("Segment must have a sequence")
	}
	seg := new(segment)
	seg.entryType = "S"
	seg.name = n
	seg.sequence = seq
	seg.length = len(seq)
	if len(optional) != 0 {
		for _, field := range optional {
			val := bytes.Split(field, []byte(":"))
			switch string(val[0]) {
			case "RC":
				seg.readCount = int(val[2][0])
			case "FC":
				seg.fragCount = int(val[2][0])
			case "KC":
				seg.kmerCount = int(val[2][0])
			case "SH":
				seg.checksum = val[2]
			case "UR":
				seg.uri = string(val[2])
			case "LN":
				continue
			default:
				return nil, fmt.Errorf("Don't recognise optional field: %v", string(field))
			}
		}
	}
	return seg, nil
}

// Returns a formatted segment line
func (self *segment) PrintGFAline() string {
	line := fmt.Sprintf("%v\t%v\t%v\tLN:i:%v", self.entryType, string(self.name), string(self.sequence), self.length)
	if self.readCount != 0 {
		line = fmt.Sprintf("%v\tRC:i:%v", line, self.readCount)
	}
	if self.fragCount != 0 {
		line = fmt.Sprintf("%v\tFC:i:%v", line, self.fragCount)
	}
	if self.kmerCount != 0 {
		line = fmt.Sprintf("%v\tKC:i:%v", line, self.kmerCount)
	}
	if self.checksum != nil {
		line = fmt.Sprintf("%v\tSH:i:%s", line, self.checksum)
	}
	if self.uri != "" {
		line = fmt.Sprintf("%v\tUR:i:%v", line, self.uri)
	}
	return line
}

// Returns the entryType field
func (self *segment) PrintType() string {
	return "segment"
}

// Adds a segment to a GFA instance
func (self *segment) Add(gfa *GFA) error {
		if _, ok := gfa.segRecord[string(self.name)]; ok {
			return fmt.Errorf("Duplicate segment name already present in GFA instance: %v", string(self.name))
		}
		gfa.segments = append(gfa.segments, self)
		gfa.segRecord[string(self.name)] = struct{}{}
		return nil
}

/*
Links are the primary mechanism to connect segments. Links connect oriented segments.
A link from A to B means that the end of A overlaps with the start of B.
If either is marked with -, we replace the sequence of the segment with its reverse complement, whereas a + indicates the segment sequence is used as-is.
The length of the overlap is determined by the CIGAR string of the link.
When the overlap is 0M the B segment follows directly after A.
When the CIGAR string is *, the nature of the overlap is not specified.
*/
type link struct {
	entryType  string
	from       []byte
	fromOrient string
	to         []byte
	toOrient   string
	overlap    string
}

// Link constructor
func NewLink(from, fOrient, to, tOrient, overlap []byte, optional ...[]byte) (*link, error) {
	if bytes.ContainsAny(from, "+-*= ") {
		return nil, fmt.Errorf("Segment name can't contain +/-/*/= or whitespace")
	}
	if bytes.ContainsAny(to, "+-*= ") {
		return nil, fmt.Errorf("Segment name can't contain +/-/*/= or whitespace")
	}
	link := new(link)
	link.from = from
	link.to = to
	link.entryType = "L"
	fori, tori := string(fOrient), string(tOrient)
	if (fori == "+") || (fori == "-") {
		link.fromOrient = fori
	} else {
		return nil, fmt.Errorf("From orientation field must be either + or -")
	}
	if (tori == "+") || (tori == "-") {
		link.toOrient = tori
	} else {
		return nil, fmt.Errorf("To orientation field must be either + or -")
	}
	link.overlap = string(overlap)
	// TODO: add optional fields...

	return link, nil
}

// Returns a formatted link line
func (self *link) PrintGFAline() string {
	line := fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v", self.entryType, string(self.from), self.fromOrient, string(self.to), self.toOrient, self.overlap)
	return line
}

// Returns the entryType field
func (self *link) PrintType() string {
	return "link"
}

// Adds a link to a GFA instance
func (self *link) Add(gfa *GFA) error {
	gfa.links = append(gfa.links, self)
	return nil
}

// containment
type containment struct {
	entryType string
}

// path
type path struct {
	entryType string
}