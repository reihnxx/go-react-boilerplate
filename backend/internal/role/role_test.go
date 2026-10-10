package role

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Kepala Sekolah": "kepala-sekolah",
		"Super  Admin!!": "super-admin",
		"2nd Operator":   "nd-operator",
		"!!!":            "role",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
