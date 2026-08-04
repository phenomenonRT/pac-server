package domainlist

import (
	"reflect"
	"testing"
)

func TestParseText(t *testing.T) {
	got := ParseText(`
# comment
domain:example.com
full:api.example.org
keyword:ignored
regexp:.*ignored.*
plain.example.net @cn
`)
	want := []string{"example.com", "api.example.org", "plain.example.net"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseText() = %#v, want %#v", got, want)
	}
}
