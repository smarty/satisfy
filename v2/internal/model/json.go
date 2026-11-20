package model

import (
	"encoding/json/v2"
	"net/url"
)

// URL is a url.URL that marshals to and from a JSON string.
type URL url.URL

// MarshalJSON has a value receiver so that a URL is marshaled as a string
// even when the struct holding it is marshaled by value.
func (this URL) MarshalJSON() ([]byte, error) {
	return json.Marshal(this.Value().String())
}

// UnmarshalJSON accepts a JSON string. A JSON null (or, for compatibility,
// the string "null") leaves the URL unchanged.
func (this *URL) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		return nil
	}
	var raw string
	if err := json.Unmarshal(p, &raw); err != nil {
		return err
	}
	if raw == "null" {
		return nil
	}
	address, err := url.Parse(raw)
	if err == nil {
		*this = URL(*address)
	}
	return err
}

// String returns the address in URL form, e.g. "gcs://bucket/prefix".
func (this URL) String() string {
	return this.Value().String()
}

func (this URL) Value() *url.URL {
	standard := url.URL(this)
	return &standard
}
