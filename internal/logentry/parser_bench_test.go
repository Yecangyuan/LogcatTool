package logentry

import "testing"

var benchLines = []string{
	"04-09 10:42:01.234  1234  5678 D MyTag   : Some debug message with content",
	"04-09 10:42:01.235 12345 56789 E AndroidRuntime: FATAL EXCEPTION: main",
	"01-01 00:00:00.000     1     1 I ActivityManager: Start proc 1234:com.example.app/u0a123 for activity",
	"12-31 23:59:59.999 32767 32767 W System.err: java.lang.NullPointerException",
	"04-09 10:42:01.234  1234  5678 V VeryLongTagNameHere: Short msg",
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, line := range benchLines {
			Parse(line)
		}
	}
}
