package downloadstation

import "testing"

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"waiting":             "waiting",
		"downloading":         "downloading",
		"paused":              "paused",
		"finishing":           "finishing",
		"finished":            "finished",
		"seeding":             "seeding",
		"error":               "error",
		"filehosting_waiting": "finishing",
		"hash_checking":       "downloading",
		"1":                   "waiting",
		"2":                   "downloading",
		"3":                   "paused",
		"4":                   "finishing",
		"5":                   "finished",
		"7":                   "seeding",
		"10":                  "error",
		"35":                  "error",
		"36":                  "unknown",
		"something_else":      "unknown",
	}
	for in, want := range cases {
		if got := NormalizeStatus(in); got != want {
			t.Fatalf("NormalizeStatus(%q)=%q want %q", in, got, want)
		}
	}
}

func TestStatusEnum(t *testing.T) {
	if got := StatusEnum("3"); got != "paused" {
		t.Fatalf("StatusEnum(3)=%q want paused", got)
	}
	if got := StatusEnum("31"); got != "extract_failed" {
		t.Fatalf("StatusEnum(31)=%q want extract_failed", got)
	}
	if got := StatusEnum("downloading"); got != "downloading" {
		t.Fatalf("StatusEnum(downloading)=%q want downloading", got)
	}
}

func TestStatusDisplay(t *testing.T) {
	if got := StatusDisplay("3"); got != "paused (3)" {
		t.Fatalf("StatusDisplay(3)=%q want paused (3)", got)
	}
	if got := StatusDisplay("99"); got != "unknown (99)" {
		t.Fatalf("StatusDisplay(99)=%q want unknown (99)", got)
	}
	if got := StatusDisplay("paused"); got != "paused" {
		t.Fatalf("StatusDisplay(paused)=%q want paused", got)
	}
}

func TestTerminalHelpers(t *testing.T) {
	if !IsTerminalSuccess("finished") || !IsTerminalSuccess("seeding") {
		t.Fatal("expected finished and seeding as terminal success")
	}
	if !IsTerminalFailure("error") {
		t.Fatal("expected error as terminal failure")
	}
}
