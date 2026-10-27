package main

import "testing"

func TestBoundedEnvironmentConfiguration(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
		bad   bool
	}{{"", 2, false}, {"1", 1, false}, {"4", 4, false}, {"0", 0, true}, {"5", 0, true}, {"two", 0, true}} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("BUILD_WORKERS", test.value)
			n, err := envInt("BUILD_WORKERS", 2, 1, 4)
			if (err != nil) != test.bad || !test.bad && n != test.want {
				t.Fatalf("got %d,%v", n, err)
			}
		})
	}
}
