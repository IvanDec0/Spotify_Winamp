package lyrics

import "testing"

func TestParseLRCMultipleTimestampsAndFractions(t *testing.T) {
	raw := "[ar:Artist]\n[ti:Title]\n[00:12.34]First line\n[00:15.5][00:20.250]Shared line\n[offset:-500]\n[01:02.003]Last line"
	lines := ParseLRC(raw)
	if len(lines) != 4 {
		t.Fatalf("ParseLRC() returned %d lines; want 4", len(lines))
	}
	want := []Line{
		{TimeMS: 11_840, Text: "First line"},
		{TimeMS: 15_000, Text: "Shared line"},
		{TimeMS: 19_750, Text: "Shared line"},
		{TimeMS: 61_503, Text: "Last line"},
	}
	for index, line := range lines {
		if line != want[index] {
			t.Fatalf("line %d = %+v; want %+v", index, line, want[index])
		}
	}
}

func TestParseLRCEmptyAndMetadata(t *testing.T) {
	if lines := ParseLRC("[ar:Artist]\n[ti:Title]\n"); lines != nil {
		t.Fatalf("ParseLRC(metadata only) = %+v; want nil", lines)
	}
	if lines := ParseLRC("plain text without timestamps"); lines != nil {
		t.Fatalf("ParseLRC(plain) = %+v; want nil", lines)
	}
}

func TestParseLRCSortsOutOfOrderLines(t *testing.T) {
	lines := ParseLRC("[00:30.00]Third\n[00:10.00]First\n[00:20.00]Second")
	if len(lines) != 3 {
		t.Fatalf("ParseLRC() returned %d lines; want 3", len(lines))
	}
	for index, want := range []string{"First", "Second", "Third"} {
		if lines[index].Text != want {
			t.Fatalf("line %d = %q; want %q", index, lines[index].Text, want)
		}
	}
}

func TestParseLRCNormalizesCRLF(t *testing.T) {
	lines := ParseLRC("[00:01.00]One\r\n[00:02.00]Two\r\n")
	if len(lines) != 2 || lines[0].Text != "One" || lines[1].Text != "Two" {
		t.Fatalf("ParseLRC(CRLF) = %+v", lines)
	}
}
