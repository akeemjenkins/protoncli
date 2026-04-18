package cerr

import "encoding/json"

// ToEnvelope returns a JSON-ready map describing the error.
func (e *Error) ToEnvelope() map[string]any {
	inner := map[string]any{
		"kind":    e.Kind.String(),
		"code":    e.Code,
		"reason":  e.Reason,
		"message": e.Message,
	}
	if e.Hint != "" {
		inner["hint"] = e.Hint
	}
	return map[string]any{"error": inner}
}

// ToJSON returns the indented JSON envelope.
func (e *Error) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e.ToEnvelope(), "", "  ")
}
