package project

import (
	"reflect"
	"testing"
)

func TestDirWorkspace_Name(t *testing.T) {
	type fields struct {
		name           string
		root           string
		maxDepth       int
		projectChecker pathChecker
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"test-1", fields{"name-1", "", 1, GitProjectChecker}, "name-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewDirWorkspace(tt.fields.name, tt.fields.root, tt.fields.maxDepth, tt.fields.projectChecker)
			if got := s.Name(); got != tt.want {
				t.Errorf("Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirWorkspace_Projects(t *testing.T) {
	type fields struct {
		name           string
		root           string
		maxDepth       int
		projectChecker pathChecker
	}
	tests := []struct {
		name   string
		fields fields
		want   []Project
	}{
		{"scan-1", fields{
			"scan-1",
			"/Users/heyu/Code/local/",
			1,
			GitProjectChecker,
		}, []Project{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewDirWorkspace(tt.fields.name, tt.fields.root, tt.fields.maxDepth, tt.fields.projectChecker)
			if got := s.Projects(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Projects() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirWorkspace_Scan(t *testing.T) {
	type fields struct {
		name           string
		root           string
		maxDepth       int
		projectChecker pathChecker
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{"scan-1", fields{
			"scan-1",
			"/Users/heyu/Code/temp/",
			1,
			GitProjectChecker,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewDirWorkspace(tt.fields.name, tt.fields.root, 1, tt.fields.projectChecker)
			s.Scan()
		})
	}
}
