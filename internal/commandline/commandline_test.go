package commandline

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "plain args",
			value: "code --reuse-window",
			want:  []string{"code", "--reuse-window"},
		},
		{
			name:  "quoted command path",
			value: `"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" --wait`,
			want:  []string{"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code", "--wait"},
		},
		{
			name:  "quoted args",
			value: `editor "arg with spaces" 'single quoted' plain\ arg`,
			want:  []string{"editor", "arg with spaces", "single quoted", "plain arg"},
		},
		{
			name:  "windows paths",
			value: `"C:\Program Files\Editor\editor.exe" C:\dev\app`,
			want:  []string{`C:\Program Files\Editor\editor.exe`, `C:\dev\app`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(tt.value)
			if err != nil {
				t.Fatalf("Split() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Split() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSplitRejectsUnterminatedQuote(t *testing.T) {
	if _, err := Split(`"code --wait`); err == nil {
		t.Fatal("Split() error = nil")
	}
}
