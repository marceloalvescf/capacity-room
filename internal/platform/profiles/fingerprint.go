package profiles

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
)

// CanonicalJSON is the JSON of d with sorted keys and no spaces, the form
// stored in the row and hashed. A nil list is written as an empty one.
func CanonicalJSON(d Document) ([]byte, error) {
	for _, l := range []*[]string{
		&d.AllowedNamespaces, &d.ExcludedContainers, &d.PermittedPeriods,
		&d.PermittedTimezones, &d.PermittedReportProfiles,
	} {
		if *l == nil {
			*l = []string{}
		}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	return json.Marshal(tree)
}

// Fingerprint is the SHA-256, in hex, of "capacityroom.profile.v1\n", the canonical
// JSON of d, "\n", and the parent revision id ("" for the first revision).
func Fingerprint(d Document, parent *uuid.UUID) string {
	body, err := CanonicalJSON(d)
	if err != nil {
		// A Document holds only strings, booleans, finite numbers and lists;
		// a non-finite number is refused by Validate before any hashing.
		body = nil
	}
	parentText := ""
	if parent != nil {
		parentText = parent.String()
	}
	sum := sha256.Sum256([]byte("capacityroom.profile.v1\n" + string(body) + "\n" + parentText))
	return hex.EncodeToString(sum[:])
}
