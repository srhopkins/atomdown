package atomdown

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// digestAlgorithm names the hash function embedded in every Core content
// digest. Naming it inside the value lets a reader recognize a future
// second algorithm without a schema change.
const digestAlgorithm = "sha256"

// digestPattern matches a well-formed Core content digest: the algorithm
// name, a colon, and the lowercase hex SHA-256 sum.
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ContentDigest returns the Core content digest for one atom's block text.
// See SPEC.md "Content digest" for the normative definition. In summary: it
// hashes the raw source bytes of the atom's block, including all
// whitespace, after normalizing CRLF and lone CR line endings to LF. Line
// ending normalization is the only transformation Core performs; nothing
// else about the bytes is altered, trimmed, or reflowed.
func ContentDigest(blockText string) string {
	normalized := normalizeLineEndings(blockText)
	sum := sha256.Sum256([]byte(normalized))
	return digestAlgorithm + ":" + hex.EncodeToString(sum[:])
}

// normalizeLineEndings rewrites CRLF and lone CR sequences to LF, so the
// same document produces the same digest whether it was checked out with
// Windows or Unix line endings.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}
