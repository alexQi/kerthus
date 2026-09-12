package file

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SafeFilename is display metadata only. It is never an object-storage path.
func SafeFilename(value string) string {
	value = path.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(value, ""))
	value = strings.Trim(value, " .")
	if value == "" || value == "/" {
		value = "attachment.bin"
	}
	for len(value) > 240 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}
