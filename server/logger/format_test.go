package logger

import (
	"reflect"
	"testing"
)

func Test_parseFormatRule(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want formatRule
	}{
		{
			arg: "[{level} - {time} - {time:2006-01-02 15:04:05} {function}:{file}:{line}] {message} {attrs}",
			want: formatRule{
				{segString, "["},
				{segLevel, ""},
				{segString, " - "},
				{segTime, ""},
				{segString, " - "},
				{segTime, "2006-01-02 15:04:05"},
				{segString, " "},
				{segFunction, ""},
				{segString, ":"},
				{segFile, ""},
				{segString, ":"},
				{segLine, ""},
				{segString, "] "},
				{segMessage, ""},
				{segString, " "},
				{segAttrs, ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFormatRule(tt.arg)

			if len(got) != len(tt.want) {
				t.Errorf("parseFormatRule().len = %v, want %v", len(got), len(tt.want))
			}

			for i, gotSeg := range got {
				wantSeg := tt.want[i]
				if !reflect.DeepEqual(gotSeg, wantSeg) {
					t.Errorf("parseFormatRule().%d = %v, want %v", i, gotSeg, wantSeg)
				}
			}
		})
	}
}
