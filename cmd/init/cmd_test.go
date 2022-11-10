package init

import (
	"bufio"
	"bytes"
	"testing"
)

func TestPrompt(t *testing.T) {
	type test struct {
		name  string
		input string
		want  string
	}

	tests := []test{
		{name: "single word", input: "test", want: "test"},
		{name: "multi word", input: "multiple words, with punctuation!", want: "multiple words, with punctuation!"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := prompt(tc.input, bufio.NewReader(bytes.NewBufferString(tc.input)))
			if result != tc.want {
				t.Fatalf(`Expected "%v", got "%v"`, tc.want, result)
			}
		})
	}
}
