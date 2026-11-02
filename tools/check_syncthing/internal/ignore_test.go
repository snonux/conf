package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseIgnoreRules(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []IgnoreRule
		wantErr string
	}{
		{
			name: "comments, label with spaces and limits",
			in:   "# known\n\n  Galaxy Tab S10 FE DCIM ; max-items=4; max-bytes=20226448\nabcd-1234\n",
			want: []IgnoreRule{
				{Folder: "Galaxy Tab S10 FE DCIM", MaxItems: 4, MaxBytes: 20226448},
				{Folder: "abcd-1234", MaxItems: noLimit, MaxBytes: noLimit},
			},
		},
		{name: "empty", in: "# nothing\n", want: nil},
		{name: "missing folder", in: "; max-items=1", wantErr: "line 1: missing folder"},
		{name: "not key=value", in: "x; max-items", wantErr: "not key=value"},
		{name: "negative", in: "x; max-bytes=-1", wantErr: "non-negative integer"},
		{name: "not a number", in: "x; max-items=four", wantErr: "non-negative integer"},
		{name: "unknown key", in: "\nx; max-files=1", wantErr: "line 2: unknown condition"},
		{
			name: "device rule next to a folder rule",
			in:   "device:  Galaxy Tab S10 FE \nGalaxy Tab S10 FE DCIM\n",
			want: []IgnoreRule{
				{Device: "Galaxy Tab S10 FE", MaxItems: noLimit, MaxBytes: noLimit},
				{Folder: "Galaxy Tab S10 FE DCIM", MaxItems: noLimit, MaxBytes: noLimit},
			},
		},
		{name: "device rule without device", in: "device:", wantErr: "device rule needs"},
		{name: "device rule with conditions", in: "device: x; max-items=1", wantErr: "no conditions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIgnoreRules(strings.NewReader(tt.in))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIgnoreRuleMatches(t *testing.T) {
	galaxy := Folder{ID: "sm-x526b-photos", Label: "Galaxy Tab S10 FE DCIM"}
	known := FolderStatus{State: "idle", NeedTotalItems: 4, NeedBytes: 20226448}
	tests := []struct {
		name   string
		rule   IgnoreRule
		folder Folder
		status FolderStatus
		want   bool
	}{
		{"label within limits", IgnoreRule{Folder: "Galaxy Tab S10 FE DCIM", MaxItems: 4, MaxBytes: 20226448}, galaxy, known, true},
		{"id without limits", IgnoreRule{Folder: "sm-x526b-photos", MaxItems: noLimit, MaxBytes: noLimit}, galaxy, FolderStatus{NeedTotalItems: 999}, true},
		{"more items than recorded", IgnoreRule{Folder: "Galaxy Tab S10 FE DCIM", MaxItems: 4, MaxBytes: noLimit}, galaxy, FolderStatus{NeedTotalItems: 5}, false},
		{"more bytes than recorded", IgnoreRule{Folder: "Galaxy Tab S10 FE DCIM", MaxItems: noLimit, MaxBytes: 20226448}, galaxy, FolderStatus{NeedBytes: 20226449}, false},
		{"other folder", IgnoreRule{Folder: "Galaxy", MaxItems: noLimit, MaxBytes: noLimit}, galaxy, known, false},
		{"device rule never matches a folder", IgnoreRule{Device: "Galaxy Tab S10 FE DCIM", MaxItems: noLimit, MaxBytes: noLimit}, galaxy, known, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.Matches(tt.folder, tt.status); got != tt.want {
				t.Fatalf("Matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadIgnoreFile(t *testing.T) {
	if rules, err := LoadIgnoreFile(""); err != nil || rules != nil {
		t.Fatalf("empty path: %v, %v", rules, err)
	}
	if _, err := LoadIgnoreFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file: want error")
	}
	path := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(path, []byte("x; bogus=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIgnoreFile(path); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("bad file: err = %v, want naming %s", err, path)
	}
}
