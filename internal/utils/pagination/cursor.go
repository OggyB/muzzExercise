package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Cursor is the opaque pagination state we encode/decode.
// ActorID + UpdatedUnix (in millis) establish a stable cursor.
type Cursor struct {
	ActorID     uint64 `json:"actor_id"`
	UpdatedUnix int64  `json:"updated_unix,omitempty"`
}

// Encode converts a Cursor into a Base64 string.
func Encode(c Cursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Decode parses a Base64 string into a Cursor.
// Empty token → empty cursor (first page).
func Decode(token string) (Cursor, error) {
	if token == "" {
		return Cursor{}, nil
	}

	b, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid pagination token")
	}

	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, fmt.Errorf("invalid pagination token")
	}
	return c, nil
}
