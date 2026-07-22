package store

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"SensorReading":  "sensorreading",
		"Sensor Reading": "sensor-reading",
		"edge-app":       "edge-app",
		"Foo/Bar Baz!":   "foo-bar-baz",
		"  Trimmed  ":    "trimmed",
		"multi__score":   "multi-score",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRawFilename(t *testing.T) {
	cases := map[string]string{
		"sql":        "sql.raw.sql",
		"protobuf":   "protobuf.raw.proto",
		"json":       "json.raw.json",
		"pydantic":   "pydantic.raw.py",
		"typescript": "typescript.raw.ts",
		"cpp_struct": "cpp_struct.raw.h",
		"weirdkind":  "weirdkind.raw.txt",
	}
	for kind, want := range cases {
		if got := RawFilename(kind); got != want {
			t.Errorf("RawFilename(%q) = %q, want %q", kind, got, want)
		}
	}
}
