// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package buildcfg

import (
	"slices"
	"strings"
	"testing"
)

// mustParseKibou parses KIBOU_EXPERIMENTS for a fixed platform with an empty
// GOEXPERIMENT, failing the test on error.
func mustParseKibou(t *testing.T, kibouExpts string) *ExperimentFlags {
	t.Helper()
	flags, err := ParseExperimentFlags("linux", "amd64", "", kibouExpts)
	if err != nil {
		t.Fatalf("ParseExperimentFlags(%q) failed: %v", kibouExpts, err)
	}
	return flags
}

func TestParseKibouExperiments(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantEnums   bool
		wantString  string   // KibouExptString: differs-from-baseline, normalized
		wantEnabled []string // KibouExptEnabled: enabled experiments, PascalCase
		wantAll     []string // KibouExptAll: every experiment, No-prefixed when off
	}{
		{
			name:        "enums",
			in:          "Enums",
			wantEnums:   true,
			wantString:  "Enums",
			wantEnabled: []string{"Enums"},
			wantAll:     []string{"Enums"},
		},
		{
			// NoEnums matches the baseline, so it is elided from String/Enabled.
			name:    "no_enums",
			in:      "NoEnums",
			wantAll: []string{"NoEnums"},
		},
		{
			name:    "none_disables_all",
			in:      "none",
			wantAll: []string{"NoEnums"},
		},
		{
			name:    "empty_is_baseline",
			in:      "",
			wantAll: []string{"NoEnums"},
		},
		{
			// A trailing comma yields an empty field, which is skipped.
			name:        "trailing_comma",
			in:          "Enums,",
			wantEnums:   true,
			wantString:  "Enums",
			wantEnabled: []string{"Enums"},
			wantAll:     []string{"Enums"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := mustParseKibou(t, tt.in)

			if got := flags.Kibou.Enums; got != tt.wantEnums {
				t.Errorf("Kibou.Enums = %v, want %v", got, tt.wantEnums)
			}
			if got := flags.KibouExptString(); got != tt.wantString {
				t.Errorf("KibouExptString() = %q, want %q", got, tt.wantString)
			}
			if got := flags.KibouExptEnabled(); !slices.Equal(got, tt.wantEnabled) {
				t.Errorf("KibouExptEnabled() = %v, want %v", got, tt.wantEnabled)
			}
			if got := flags.KibouExptAll(); !slices.Equal(got, tt.wantAll) {
				t.Errorf("KibouExptAll() = %v, want %v", got, tt.wantAll)
			}
		})
	}
}

func TestParseKibouExperimentsErrors(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantContain []string
	}{
		{
			name:        "single_unsupported",
			in:          "Bogus",
			wantContain: []string{"unsupported value for KIBOU_EXPERIMENTS Bogus", "Supported value(s): Enums"},
		},
		{
			// Multiple unknowns switch to the plural form and are sorted.
			name:        "multiple_unsupported",
			in:          "Zeta,Alpha",
			wantContain: []string{"unsupported values for KIBOU_EXPERIMENTS Alpha, Zeta", "Supported value(s): Enums"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseExperimentFlags("linux", "amd64", "", tt.in)
			if err == nil {
				t.Fatalf("ParseExperimentFlags(%q) succeeded, want error", tt.in)
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}
