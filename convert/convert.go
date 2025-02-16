package convert

import (
	"reflect"
	"strings"
	"time"
)

const DefaultTimeFormat = time.RFC3339Nano

// getStructFieldKey returns the key name derived from the field's JSON tag.
// If no JSON tag is present, the field name is used. If the tag is "-", an empty string is returned.
func getStructFieldKey(sf reflect.StructField) string {
	tag := sf.Tag.Get("json")
	if tag == "" {
		return sf.Name
	}
	key := strings.Split(tag, ",")[0]
	if key == "-" {
		return ""
	}
	return key
}
