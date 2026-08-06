package netiface

import "testing"

func TestListAlwaysIncludesAllInterfacesOption(t *testing.T) {
	options := List()

	found := false
	for _, opt := range options {
		if opt.IP == "0.0.0.0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("List() = %#v, want an 0.0.0.0 option", options)
	}
}

func TestWithCurrentAddsMissingIP(t *testing.T) {
	options := WithCurrent(nil, []string{"10.0.0.5"})
	if len(options) != 1 || options[0].IP != "10.0.0.5" {
		t.Fatalf("WithCurrent() = %#v, want a single 10.0.0.5 option", options)
	}
}

func TestWithCurrentKeepsExistingList(t *testing.T) {
	base := []Option{{Label: "lo (127.0.0.1)", IP: "127.0.0.1"}}
	options := WithCurrent(base, []string{"127.0.0.1"})
	if len(options) != 1 {
		t.Fatalf("WithCurrent() = %#v, want unchanged single-option list", options)
	}
}
