package main

import (
	"errors"
	"testing"
)

func TestSettingArgs(t *testing.T) {
	for _, cmd := range []string{"shuffle", "autoplay"} {
		for _, value := range []string{"on", "off"} {
			t.Run(cmd+"/"+value, func(t *testing.T) {
				params, err := commandArgs(cmd, []string{value})
				if err != nil || params["enabled"] != (value == "on") {
					t.Fatalf("commandArgs = %v, %v", params, err)
				}
			})
		}
		for _, args := range [][]string{nil, {"true"}, {"toggle"}, {"on", "off"}} {
			if _, err := commandArgs(cmd, args); !errors.Is(err, errUsage) {
				t.Errorf("commandArgs(%q, %v): got %v, want usage error", cmd, args, err)
			}
		}
	}
}
