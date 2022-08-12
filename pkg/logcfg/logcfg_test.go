package logcfg

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestVerbosityParsing(t *testing.T) {
	var tests = []struct {
		name      string
		input     string
		changed   bool
		expected  log.Level
		expectErr bool
	}{
		{
			name:      "Not a real log level",
			input:     "fake",
			changed:   true,
			expected:  defaultLogLevel,
			expectErr: true,
		},
		{
			name:      "Info log level",
			input:     "info",
			changed:   true,
			expected:  log.InfoLevel,
			expectErr: false,
		},
		{
			name:      "No log level and unchanged",
			input:     "",
			changed:   false,
			expected:  log.WarnLevel,
			expectErr: false,
		},
		{
			name:      "No log level and changed",
			input:     "",
			changed:   true,
			expected:  log.DebugLevel,
			expectErr: false,
		},
		{
			name:      "Numbered log level",
			input:     "4",
			changed:   true,
			expected:  log.InfoLevel,
			expectErr: false,
		},
		{
			name:      "Numbered log level out of bounds",
			input:     "99",
			changed:   true,
			expected:  log.TraceLevel,
			expectErr: false,
		},
		{
			name:      "Negative log level",
			input:     "-3",
			changed:   true,
			expected:  log.WarnLevel,
			expectErr: false,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			output, err := parseVerbosity(test.input, test.changed)
			if !test.expectErr && err != nil {
				t.Errorf("Unexpected Error: %v", err)
			}

			if output != test.expected {
				t.Errorf("Unexpected Output: %v, Expected: %v", output, test.expected)
			}
		})
	}
}
