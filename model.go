package atomdown

// Position identifies a byte offset and its one-based line and column.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Range identifies a half-open source range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Attribute is an XML attribute not defined by Atomdown Core.
type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Atom is one explicit or implicit top-level Markdown unit.
type Atom struct {
	ID         string      `json:"id,omitempty"`
	Slug       string      `json:"slug,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
	Marker     *Range      `json:"marker,omitempty"`
	Content    Range       `json:"content"`
	NodeType   string      `json:"nodeType"`
	Text       string      `json:"text"`
	Implicit   bool        `json:"implicit"`
	GroupID    string      `json:"groupId,omitempty"`
}

// AtomGroup is a contiguous, ordered collection of explicit atoms.
type AtomGroup struct {
	ID         string      `json:"id"`
	Slug       string      `json:"slug,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
	Marker     Range       `json:"marker"`
	EndMarker  *Range      `json:"endMarker,omitempty"`
	AtomIDs    []string    `json:"atomIds,omitempty"`
}

// Severity describes the effect of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic describes a syntax or semantic defect.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Position Position `json:"position"`
	Fix      string   `json:"fix,omitempty"`
}

// Document is the parsed Atomdown view of a Markdown source file.
type Document struct {
	Declared    bool         `json:"declared"`
	Version     string       `json:"version,omitempty"`
	Attributes  []Attribute  `json:"attributes,omitempty"`
	Atoms       []Atom       `json:"atoms"`
	Groups      []AtomGroup  `json:"groups,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// HasErrors reports whether the document contains an error diagnostic.
func (d Document) HasErrors() bool {
	for _, diagnostic := range d.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}
