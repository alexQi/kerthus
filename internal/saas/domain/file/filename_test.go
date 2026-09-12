package file

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSafeFilename(t *testing.T) {
	for _, v := range []struct{ in, out string }{{`C:\docs\报告.pdf`, "报告.pdf"}, {"../../财务\r\n报告.pdf", "财务报告.pdf"}, {"\x00\u202e", "attachment.bin"}, {"..", "attachment.bin"}} {
		if got := SafeFilename(v.in); got != v.out {
			t.Fatalf("%q => %q", v.in, got)
		}
	}
	got := SafeFilename(strings.Repeat("报", 200) + ".pdf")
	if len(got) > 240 || !utf8.ValidString(got) {
		t.Fatal("invalid truncated filename")
	}
}
