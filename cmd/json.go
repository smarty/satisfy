package cmd

import (
	"net/url"
	"strings"
)

type URL url.URL

func (this *URL) MarshalJSON() ([]byte, error) {
	return []byte(`"` + this.Value().String() + `"`), nil
}

func (this *URL) UnmarshalJSON(p []byte) error {
	raw := string(p)
	if raw == `"null"` {
		return nil
	}
	raw = strings.Trim(raw, "\"")
	address, err := url.Parse(raw)
	if err == nil {
		*this = URL(*address)
	}
	return err
}

func (this URL) Value() *url.URL {
	standard := url.URL(this)
	return &standard
}
