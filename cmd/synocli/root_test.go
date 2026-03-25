package main

import (
	"testing"

	"synocli/internal/config"
)

func TestValidateAuthOptionsPasswordStdinConflict(t *testing.T) {
	ac := &appContext{opts: config.GlobalOptions{Password: "x", PasswordStdin: true}}
	if err := ac.validateAuthOptions(); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestValidateAuthOptionsCredentialsFileExclusive(t *testing.T) {
	ac := &appContext{opts: config.GlobalOptions{CredentialsFile: "/tmp/creds.env", Password: "x"}}
	if err := ac.validateAuthOptions(); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestValidateAuthOptionsCredentialsFileOnly(t *testing.T) {
	ac := &appContext{opts: config.GlobalOptions{CredentialsFile: "/tmp/creds.env"}}
	if err := ac.validateAuthOptions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
