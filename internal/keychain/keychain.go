// Package keychain stores Connection passwords as macOS Keychain generic
// passwords (service "yaam", account = Connection name) through
// /usr/bin/security, the way tantalum does.
package keychain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// ErrNotFound means the Keychain holds no password for the account.
var ErrNotFound = errors.New("no password in Keychain")

// Store is the password store yaam uses; tests use Fake.
type Store interface {
	Get(account string) (string, error) // ErrNotFound when absent
	Has(account string) (bool, error)
	Set(account, password string) error
	Delete(account string) error // no error when absent
}

const (
	service        = "yaam"
	securityTool   = "/usr/bin/security"
	errSecNotFound = 44 // security(1) exit status for a missing item
)

// Security is the Store backed by the login Keychain.
type Security struct{}

func validAccount(account string) error {
	if account == "" || strings.ContainsAny(account, "\"\\\n ") {
		return fmt.Errorf("invalid Keychain account %q", account)
	}
	return nil
}

// ValidPassword reports whether password can round-trip through
// `security find-generic-password -w`, which prints anything outside
// printable ASCII as hex.
func ValidPassword(password string) bool {
	for i := range len(password) {
		if password[i] < 0x20 || password[i] > 0x7e {
			return false
		}
	}
	return true
}

func (Security) Get(account string) (string, error) {
	if err := validAccount(account); err != nil {
		return "", err
	}
	out, err := exec.Command(securityTool, "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", securityError("read", err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func (Security) Has(account string) (bool, error) {
	if err := validAccount(account); err != nil {
		return false, err
	}
	err := exec.Command(securityTool, "find-generic-password", "-s", service, "-a", account).Run()
	if err == nil {
		return true, nil
	}
	if err := securityError("read", err); !errors.Is(err, ErrNotFound) {
		return false, err
	}
	return false, nil
}

func (Security) Set(account, password string) error {
	if err := validAccount(account); err != nil {
		return err
	}
	if !ValidPassword(password) {
		return errors.New("password must be printable ASCII")
	}
	// `security -i` reads the command from stdin, which keeps the password
	// out of argv where any local process could see it via ps; -X takes it
	// as hex so no quoting is involved.
	cmd := exec.Command(securityTool, "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -s %s -a \"%s\" -X %s\n",
		service, account, hex.EncodeToString([]byte(password))))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write Keychain: %v: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func (Security) Delete(account string) error {
	if err := validAccount(account); err != nil {
		return err
	}
	_, err := exec.Command(securityTool, "delete-generic-password", "-s", service, "-a", account).Output()
	if err == nil {
		return nil
	}
	if err := securityError("delete", err); !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

func securityError(op string, err error) error {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("%s Keychain: %w", op, err)
	}
	if exitErr.ExitCode() == errSecNotFound {
		return ErrNotFound
	}
	return fmt.Errorf("%s Keychain: %s", op, bytes.TrimSpace(exitErr.Stderr))
}

// Fake is an in-memory Store for tests.
type Fake struct {
	mu sync.Mutex
	m  map[string]string
}

func (f *Fake) Get(account string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.m[account]
	if !ok {
		return "", ErrNotFound
	}
	return p, nil
}

func (f *Fake) Has(account string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.m[account]
	return ok, nil
}

func (f *Fake) Set(account, password string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[account] = password
	return nil
}

func (f *Fake) Delete(account string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, account)
	return nil
}
