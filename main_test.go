package main

import "testing"

func TestParseLaunchOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want launchOptions
		err  bool
	}{
		{name: "defaults", want: launchOptions{}},
		{name: "web positional", args: []string{"--web", `D:\stories`}, want: launchOptions{Web: true, DataDir: `D:\stories`}},
		{name: "data dir", args: []string{"--data-dir", `D:\data`}, want: launchOptions{DataDir: `D:\data`}},
		{name: "equals", args: []string{`--data-dir=D:\data`}, want: launchOptions{DataDir: `D:\data`}},
		{name: "duplicate", args: []string{"--data-dir", "a", "b"}, err: true},
		{name: "unknown", args: []string{"--gui"}, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLaunchOptions(tt.args)
			if (err != nil) != tt.err {
				t.Fatalf("error = %v", err)
			}
			if tt.err {
				return
			}
			if got != tt.want {
				t.Fatalf("got %#v want %#v", got, tt.want)
			}
		})
	}
}
