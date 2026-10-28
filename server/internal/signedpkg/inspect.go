package signedpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// VerificationStatus classifies how much a bundle can be trusted.
type VerificationStatus string

const (
	StatusVerified   VerificationStatus = "verified"    // signature valid, key pinned
	StatusUnknownKey VerificationStatus = "unknown-key" // signature carrier names an unpinned key
	StatusUnsigned   VerificationStatus = "unsigned"    // no bundle.json
	StatusInvalid    VerificationStatus = "invalid"     // signature/digest mismatch
)

// Inspection is the non-fatal trust report for one bundle.
type Inspection struct {
	Status  VerificationStatus
	Meta    BundleMeta // zero KeyID/Signature for unsigned bundles
	Digest  string     // content digest over every entry except bundle.json
	Reason  string     // human-readable cause for unknown-key/invalid
	ZipSHA  string     // SHA-256 of the whole zip file
	HasMeta bool
}

// Inspect classifies a bundle without extracting it. Unreadable zips still
// return an error; trust failures are reported through Status/Reason.
func Inspect(zipPath string, ring Keyring) (Inspection, error) {
	insp := Inspection{}
	raw, err := ReadEntry(zipPath, "bundle.json", 1<<20)
	switch {
	case err == nil:
		insp.HasMeta = true
		if jerr := json.Unmarshal(raw, &insp.Meta); jerr != nil {
			insp.Status, insp.Reason = StatusInvalid, "bundle.json is not valid JSON"
			return finishInspection(insp, zipPath)
		}
		if insp.Meta.Name == "" || insp.Meta.Version == "" || insp.Meta.KeyID == "" || insp.Meta.Signature == "" {
			insp.Status, insp.Reason = StatusInvalid, "bundle.json is missing name/version/keyId/signature"
			return finishInspection(insp, zipPath)
		}
	case errors.Is(err, os.ErrNotExist):
		insp.Status = StatusUnsigned
		return finishInspection(insp, zipPath)
	default:
		return insp, err
	}

	digest, err := ContentDigest(zipPath, "bundle.json")
	if err != nil {
		return insp, err
	}
	insp.Digest = digest
	if _, ok := ring[insp.Meta.KeyID]; !ok {
		insp.Status = StatusUnknownKey
		insp.Reason = fmt.Sprintf("unknown signing key %q", insp.Meta.KeyID)
		return finishInspection(insp, zipPath)
	}
	sig, err := DecodeSignature(insp.Meta.Signature)
	if err != nil {
		insp.Status, insp.Reason = StatusInvalid, err.Error()
		return finishInspection(insp, zipPath)
	}
	if err := ring.Verify(insp.Meta.KeyID, BundlePayload(insp.Meta.Name, insp.Meta.Version, digest), sig); err != nil {
		insp.Status, insp.Reason = StatusInvalid, err.Error()
		return finishInspection(insp, zipPath)
	}
	insp.Status = StatusVerified
	return finishInspection(insp, zipPath)
}

func finishInspection(insp Inspection, zipPath string) (Inspection, error) {
	if insp.Digest == "" {
		digest, err := ContentDigest(zipPath, "bundle.json")
		if err != nil {
			return insp, err
		}
		insp.Digest = digest
	}
	sha, err := SHA256File(zipPath)
	if err != nil {
		return insp, err
	}
	insp.ZipSHA = sha
	return insp, nil
}
