package goji

import (
	"net/url"
	"testing"
)

func TestHex(t *testing.T) {
	tests := []struct {
		input byte
		ishex bool
		unhex byte
	}{
		{'0', true, 0},
		{'4', true, 4},
		{'a', true, 10},
		{'F', true, 15},
		{'h', false, 0},
		{'^', false, 0},
	}

	for _, test := range tests {
		if actual := ishex(test.input); actual != test.ishex {
			t.Errorf("ishex(%v) == %v, expected: %v", test.input, actual, test.ishex)
		}
		if actual := unhex(test.input); actual != test.unhex {
			t.Errorf("unhex(%v) == %v, expected: %v", test.input, actual, test.unhex)
		}
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		input  string
		err    error
		output string
	}{
		{"hello", nil, "hello"},
		{"file%20one%26two", nil, "file one&two"},
		{"one/two%2fthree", nil, "one/two/three"},
		{"this%20is%0not%valid", url.EscapeError("%0n"), ""},
	}

	for _, test := range tests {
		if actual, err := unescape(test.input); err != test.err {
			t.Errorf("unescape(%q) had err %v, expected: %q", test.input, err, test.err)
		} else if actual != test.output {
			t.Errorf("unescape(%q) = %q, expected: %q)", test.input, actual, test.output)
		}
	}
}
