package main

import "testing"

func TestTibiaEmail(t *testing.T) {
	if got := tibiaEmail("Joao"); got != "joao@luxviewot.com" {
		t.Fatalf("got %s", got)
	}
}

func TestRakionLogin(t *testing.T) {
	if got := rakionLogin("hello_world_xx"); got != "helloworldx" {
		t.Fatalf("got %q", got)
	}
}

func TestRakionPassword(t *testing.T) {
	if got := rakionPassword("Secret99Xxxx"); got != "secret99xxx" {
		t.Fatalf("got %q", got)
	}
}
