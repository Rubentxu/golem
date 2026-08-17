package supplychain

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name:    "valid simple purl",
			raw:     "pkg:npm/lodash@4.17.21",
			want:    "pkg:npm/lodash@4.17.21",
			wantErr: false,
		},
		{
			name:    "uppercase type normalized",
			raw:     "pkg:NPM/lodash@4.17.21",
			want:    "pkg:npm/lodash@4.17.21",
			wantErr: false,
		},
		{
			name:    "uppercase scheme normalized",
			raw:     "PKG:npm/lodash@4.17.21",
			want:    "pkg:npm/lodash@4.17.21",
			wantErr: false,
		},
		{
			name:    "mixed case type normalized name preserved",
			raw:     "pkg:NpM/LodAsH@4.17.21",
			want:    "pkg:npm/LodAsH@4.17.21",
			wantErr: false,
		},
		{
			name:    "with namespace",
			raw:     "pkg:golang/github.com/example/lib@v1.0.0",
			want:    "pkg:golang/github.com/example/lib@v1.0.0",
			wantErr: false,
		},
		{
			name:    "with qualifiers",
			raw:     "pkg:npm/%40babel/runtime@7.21.0?registry=https://registry.npmjs.org",
			want:    "pkg:npm/@babel/runtime@7.21.0?registry=https://registry.npmjs.org",
			wantErr: false,
		},
		{
			name:    "whitespace trimmed",
			raw:     "  pkg:npm/lodash@4.17.21  ",
			want:    "pkg:npm/lodash@4.17.21",
			wantErr: false,
		},
		{
			name:    "missing pkg prefix",
			raw:     "npm/lodash@4.17.21",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty purl",
			raw:     "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			raw:     "   ",
			want:    "",
			wantErr: true,
		},
		{
			name:    "pkg colon only",
			raw:     "pkg:",
			want:    "",
			wantErr: true,
		},
		{
			name:    "no name",
			raw:     "pkg:golang/",
			want:    "",
			wantErr: true,
		},
		{
			name:    "purl without version allowed",
			raw:     "pkg:npm/lodash",
			want:    "pkg:npm/lodash",
			wantErr: false,
		},
		{
			name:    "percent encoded namespace decoded",
			raw:     "pkg:npm/%40babel%2Fhelpers@7.21.0",
			want:    "pkg:npm/@babel/helpers@7.21.0",
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatalf("Normalize(%q): expected error, got nil", tc.raw)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Normalize(%q): unexpected error: %v", tc.raw, err)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSynthetic(t *testing.T) {
	cases := []struct {
		name    string
		rawName string
		version string
		want    string
	}{
		{
			name:    "normal name and version",
			rawName: "lodash",
			version: "4.17.21",
			want:    "pkg:generic/lodash@4.17.21",
		},
		{
			name:    "empty name uses unknown",
			rawName: "",
			version: "1.0.0",
			want:    "pkg:generic/unknown@1.0.0",
		},
		{
			name:    "empty version uses unknown",
			rawName: "mylib",
			version: "",
			want:    "pkg:generic/mylib@unknown",
		},
		{
			name:    "both empty",
			rawName: "",
			version: "",
			want:    "pkg:generic/unknown@unknown",
		},
		{
			name:    "name with whitespace trimmed",
			rawName: "  mylib  ",
			version: "2.0",
			want:    "pkg:generic/mylib@2.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Synthetic(tc.rawName, tc.version)
			if got != tc.want {
				t.Errorf("Synthetic(%q, %q) = %q, want %q", tc.rawName, tc.version, got, tc.want)
			}
		})
	}
}
