package api

import (
	"encoding/json"
	"io"
)

func decodeJSONObject(body io.Reader) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(body)
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, false
	}
	return obj, true
}
