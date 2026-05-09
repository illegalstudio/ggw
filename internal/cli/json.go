package cli

import (
	"encoding/json"
	"os"
)

// jsonOutput is bound to the global --json persistent flag in root.go.
var jsonOutput bool

// emitJSON writes v as indented JSON to stdout.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// maybeJSON emits v as JSON if --json is set, returning (true, err) so the
// caller can short-circuit. Otherwise returns (false, nil) and the caller
// proceeds with human-readable output.
//
// Idiomatic usage:
//
//	if done, err := maybeJSON(data); done {
//	    return err
//	}
func maybeJSON(v any) (bool, error) {
	if !jsonOutput {
		return false, nil
	}
	return true, emitJSON(v)
}

// errString turns an error into a JSON-friendly string ("" if nil).
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
