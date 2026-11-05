package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

// Network extracts only the app-owned network field, without exposing values in errors.
func Network(data []byte) (json.RawMessage, error) {
	obj, err := document(data)
	if err != nil {
		return nil, err
	}
	network, ok := obj["network_config"]
	if !ok {
		return nil, errors.New("missing network field")
	}
	if _, err := document(network); err != nil {
		return nil, errors.New("invalid network field")
	}
	return network, nil
}

// Merge restores only the network field; invalid latest JSON uses the private baseline.
func Merge(latest, baseline, network []byte) ([]byte, bool, error) {
	obj, err := document(latest)
	fallback := err != nil
	if fallback {
		obj, err = document(baseline)
	}
	if err != nil {
		return nil, fallback, errors.New("no valid configuration safety copy")
	}
	if _, err := document(network); err != nil {
		return nil, fallback, errors.New("invalid saved network field")
	}
	obj["network_config"] = json.RawMessage(network)
	result, err := json.Marshal(obj)
	return result, fallback, err
}

// Digest compares semantic JSON objects independent of whitespace and field order.
func Digest(raw []byte) (string, error) {
	obj, err := document(raw)
	if err != nil {
		return "", err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", errors.New("invalid JSON object")
	}
	_ = obj
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", errors.New("invalid JSON object")
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func document(data []byte) (map[string]json.RawMessage, error) {
	// Reject duplicate keys rather than ambiguously pinning or preserving them.
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid JSON object")
	}
	obj := make(map[string]json.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, errors.New("invalid JSON key")
		}
		name, ok := key.(string)
		if !ok {
			return nil, errors.New("invalid JSON key")
		}
		if _, exists := obj[name]; exists {
			return nil, errors.New("duplicate JSON key")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errors.New("invalid JSON value")
		}
		obj[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, errors.New("invalid JSON object end")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing JSON data")
	}
	return obj, nil
}
