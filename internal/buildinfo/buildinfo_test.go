package buildinfo

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{name: "version and commit", version: "0.2.0", commit: "abc1234", want: "0.2.0 (abc1234)"},
		{name: "empty commit", version: "0.2.0", commit: "", want: "0.2.0"},
		{name: "unknown commit", version: "0.2.0", commit: "unknown", want: "0.2.0"},
		{name: "development defaults", version: "dev", commit: "unknown", want: "dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previousVersion, previousCommit := Version, Commit
			defer func() {
				Version, Commit = previousVersion, previousCommit
			}()
			Version, Commit = test.version, test.commit
			if got := String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
