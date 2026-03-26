package downloadstation

import "testing"

func TestErrorMessageTaskCodes(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{400, "file upload failed"},
		{401, "max number of tasks reached"},
		{402, "destination denied"},
		{403, "destination does not exist"},
		{404, "invalid task id"},
		{405, "invalid task action"},
		{406, "no default destination"},
		{407, "set destination failed"},
		{408, "file does not exist"},
	}
	for _, tc := range cases {
		if got := ErrorMessage(tc.code); got != tc.want {
			t.Fatalf("ErrorMessage(%d)=%q want %q", tc.code, got, tc.want)
		}
	}
}
