package tools

import (
	"encoding/json"
	"errors"
	"fmt"

	libregraph "github.com/owncloud/libre-graph-api-go"
)

// sdkConvert maps a libre-graph SDK model onto a tool's local output struct via
// a JSON round-trip. The SDK models and the local structs share json tags for
// the fields the tools expose, so this preserves each tool's JSON output
// contract without hand-writing field-by-field copies.
func sdkConvert[T any](src any) (T, error) {
	var dst T
	b, err := json.Marshal(src)
	if err != nil {
		return dst, fmt.Errorf("encoding SDK response: %w", err)
	}
	if err := json.Unmarshal(b, &dst); err != nil {
		return dst, fmt.Errorf("decoding SDK response: %w", err)
	}
	return dst, nil
}

// sdkError surfaces the oCIS error body carried by a libre-graph SDK error, so
// callers see the same level of detail as the hand-rolled client's
// errorFromResponse instead of a bare status string.
func sdkError(err error) error {
	if err == nil {
		return nil
	}
	var ge libregraph.GenericOpenAPIError
	if errors.As(err, &ge) && len(ge.Body()) > 0 {
		return fmt.Errorf("%s: %s", ge.Error(), string(ge.Body()))
	}
	return err
}
