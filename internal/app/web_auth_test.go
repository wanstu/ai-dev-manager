package app

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestWebAdminInitializationIsAtomicAndPasswordsAreHashed(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, username := range []string{"admin-one", "admin-two"} {
		username := username
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.InitializeWebAdmin(username, "correct-horse-battery")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	successes := 0
	alreadyInitialized := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrWebAdminAlreadyInitialized):
			alreadyInitialized++
		default:
			t.Fatalf("unexpected initialization error: %v", err)
		}
	}
	if successes != 1 || alreadyInitialized != 1 {
		t.Fatalf("successes=%d already_initialized=%d", successes, alreadyInitialized)
	}

	state, err := service.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.WebAccess.Admin == nil {
		t.Fatal("web admin was not persisted")
	}
	if state.WebAccess.Admin.PasswordHash == "correct-horse-battery" {
		t.Fatal("password must not be stored in plaintext")
	}
	if _, err := service.AuthenticateWebAdmin(state.WebAccess.Admin.Username, "correct-horse-battery"); err != nil {
		t.Fatalf("stored admin cannot authenticate: %v", err)
	}
	if _, err := service.AuthenticateWebAdmin(state.WebAccess.Admin.Username, "wrong-password"); !errors.Is(err, ErrWebInvalidCredentials) {
		t.Fatalf("wrong password error=%v", err)
	}
}

func TestWebAdminValidation(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	if _, err := service.InitializeWebAdmin("ab", "long-enough"); err == nil {
		t.Fatal("short username should fail")
	}
	if _, err := service.InitializeWebAdmin("admin", "short"); err == nil {
		t.Fatal("short password should fail")
	}
}
