package scan

import (
	"fmt"
	"strings"
	"testing"
)

func TestNativeDirectiveMaskPreservesOperandsAndMasksRawLiterals(t *testing.T) {
	for _, prefix := range []string{"", "u8", "u", "U", "L"} {
		body := "#include \"service.h\"\nconst char *text = " + prefix + "R\"guard(\n#include \"fake.h\"\n)guard\";\nvoid run();\n"
		masked := nativeDirectiveMask(body)
		if !strings.HasPrefix(masked, "#include \"service.h\"\n") || strings.Contains(masked, "fake.h") || !strings.Contains(masked, "void run();") {
			t.Fatalf("prefix %q: incorrect directive mask: %q", prefix, masked)
		}
		if len(masked) != len(body) || strings.Count(masked, "\n") != strings.Count(body, "\n") {
			t.Fatalf("prefix %q: source positions changed", prefix)
		}
	}
}

// Generated native sources often contain long runs without any raw literal.
func BenchmarkNativeDirectiveMaskGeneratedSource(b *testing.B) {
	for _, size := range []int{4096, 65536} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			body := strings.Repeat("void generated() { return; }\n", size/28)
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				nativeDirectiveMask(body)
			}
		})
	}
}
