package service

import "testing"

func TestValidatePlayerUsername(t *testing.T) {
	if err := ValidatePlayerUsername("ab"); err == nil {
		t.Fatal("expected short username to fail")
	}
	if err := ValidatePlayerUsername("joao_ok"); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePlayerPassword(t *testing.T) {
	if err := ValidatePlayerPassword("short"); err == nil {
		t.Fatal("expected short password to fail")
	}
	if err := ValidatePlayerPassword("longenough"); err != nil {
		t.Fatal(err)
	}
}
