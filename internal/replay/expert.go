package replay

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

// ExpertProof contains checksum-only evidence; the generated inputs are ignored.
type ExpertProof struct {
	SourceSHA256, ProgramSHA256, RecordingSHA256, TerminalSHA256 string
	Fields                                                       uint64
	Players, Deaths, Score                                       int
	ClearedMask                                                  uint8
	Options                                                      engine.Options
}

//go:embed expert.json
var expertProofJSON []byte

// ExpertMetadata returns a value copy of the selected native expert proof.
func ExpertMetadata() ExpertProof {
	var proof ExpertProof
	if err := json.Unmarshal(expertProofJSON, &proof); err != nil {
		panic(fmt.Sprintf("invalid built-in expert metadata: %v", err))
	}
	return proof
}

// CheckExpertData rejects a different resident program or nonstandard defaults.
// The demo itself always starts with these options, independently of user settings.
func CheckExpertData(data *engine.Data) error {
	proof := ExpertMetadata()
	if data == nil || hash(data.Loader) != proof.ProgramSHA256 {
		return fmt.Errorf("expert demo requires the verified original resident program")
	}
	if data.Options != proof.Options {
		return fmt.Errorf("expert demo requires the verified original default options")
	}
	return nil
}

// LoadExpert loads and checks the locally generated input resource once.
// Runtime playback only calls Cursor.Next and the ordinary native Engine.Tick.
func LoadExpert(files fs.FS) (*Cursor, error) {
	encoded, err := fs.ReadFile(files, "expert.bsinput")
	if err != nil {
		return nil, fmt.Errorf("load expert demo (run make assets): %w", err)
	}
	recording, err := DecodeExpert(encoded)
	if err != nil {
		return nil, err
	}
	return NewCursor(recording)
}

// DecodeExpert enforces the published checksum, provenance and input length.
func DecodeExpert(encoded []byte) (Recording, error) {
	proof := ExpertMetadata()
	if hash(encoded) != proof.RecordingSHA256 {
		return Recording{}, fmt.Errorf("expert demo input checksum mismatch; regenerate with go run ./cmd/demo")
	}
	recording, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		return Recording{}, err
	}
	if hex.EncodeToString(recording.Source[:]) != proof.SourceSHA256 || recording.Fields != proof.Fields {
		return Recording{}, fmt.Errorf("expert demo provenance or length mismatch")
	}
	return recording, nil
}

func hash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
