package atomdown

import "testing"

func TestContentDigestIncludesInteriorWhitespace(t *testing.T) {
	one := ContentDigest("Para one.")
	two := ContentDigest("Para  one.")
	if one == two {
		t.Fatalf("digest ignored a whitespace-only difference: both hashed to %s", one)
	}
}

func TestContentDigestIncludesTrailingHardBreakSpaces(t *testing.T) {
	noBreak := ContentDigest("line one\nline two")
	hardBreak := ContentDigest("line one  \nline two")
	if noBreak == hardBreak {
		t.Fatalf("digest ignored trailing hard-break spaces: both hashed to %s", noBreak)
	}
}

func TestContentDigestNormalizesCRLFToLF(t *testing.T) {
	lf := ContentDigest("line one\nline two")
	crlf := ContentDigest("line one\r\nline two")
	if lf != crlf {
		t.Fatalf("CRLF and LF of identical content disagreed: %s vs %s", lf, crlf)
	}
}

func TestContentDigestNormalizesLoneCRToLF(t *testing.T) {
	lf := ContentDigest("line one\nline two")
	cr := ContentDigest("line one\rline two")
	if lf != cr {
		t.Fatalf("lone CR and LF of identical content disagreed: %s vs %s", lf, cr)
	}
}

func TestContentDigestIsDeterministic(t *testing.T) {
	first := ContentDigest("Para one.")
	second := ContentDigest("Para one.")
	if first != second {
		t.Fatalf("digest was not deterministic: %s vs %s", first, second)
	}
}

func TestContentDigestHasTheDocumentedShape(t *testing.T) {
	digest := ContentDigest("Para one.")
	if !digestPattern.MatchString(digest) {
		t.Fatalf("digest %q does not match sha256:<64 hex characters>", digest)
	}
}
