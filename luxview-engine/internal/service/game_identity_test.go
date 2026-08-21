package service

import (
	"testing"
)

func TestSHA1Hex123456(t *testing.T) {
	got := SHA1Hex("123456")
	want := "7c4a8d09ca3762af61e59520943dc26494f8941b"
	if got != want {
		t.Fatalf("SHA1Hex(123456) = %s, want %s", got, want)
	}
}

func TestMySQLPassword123456(t *testing.T) {
	got := MySQLPassword("123456")
	want := "*6BB4837EB74329105EE4568DDA7DC67ED2CA2AD9"
	if got != want {
		t.Fatalf("MySQLPassword(123456) = %s, want %s", got, want)
	}
}

func TestTibiaEmail(t *testing.T) {
	if got := TibiaEmail("TestUser"); got != "testuser@luxviewot.com" {
		t.Fatalf("got %s", got)
	}
}

func TestTibiaCharacterName(t *testing.T) {
	if got := TibiaCharacterName("testando"); got != "Testando" {
		t.Fatalf("got %s", got)
	}
	if got := TibiaCharacterName("x"); got != "Herox" {
		t.Fatalf("short got %s", got)
	}
}

func TestRakionLoginStripsAndClips(t *testing.T) {
	if got := RakionLogin("hello_world_xx"); got != "helloworldx" {
		t.Fatalf("got %q", got)
	}
	if got := RakionLogin("testando"); got != "testando" {
		t.Fatalf("got %q", got)
	}
}

func TestRakionPasswordLowerClip(t *testing.T) {
	if got := RakionPassword("AbcdefghijkZ"); got != "abcdefghijk" {
		t.Fatalf("got %q", got)
	}
}

func TestMysqlQuoteEscapes(t *testing.T) {
	if got := mysqlQuote(`a'b\c`); got != `'a\'b\\c'` {
		t.Fatalf("got %s", got)
	}
}
