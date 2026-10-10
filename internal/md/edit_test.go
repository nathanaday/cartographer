package md

import "testing"

func TestAppendToSection(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"empty, then a heading", "## A\n\n## B\n", "## A\n\nx\n\n## B\n"},
		{"empty, no blank line", "## A\n## B\n", "## A\n\nx\n\n## B\n"},
		{"empty, at the end", "## A\n", "## A\n\nx\n"},
		{"empty, blank lines at the end", "## A\n\n\n", "## A\n\nx\n"},
		{"with lines, then a heading", "## A\n\n- a\n\n## B\n", "## A\n\n- a\nx\n\n## B\n"},
		{"with lines, at the end", "## A\n\n- a\n", "## A\n\n- a\nx\n"},
		{"missing", "# T\n", "# T\n\n## A\n\nx\n"},
	}
	for _, c := range cases {
		if got := AppendToSection(c.text, "A", "x"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
