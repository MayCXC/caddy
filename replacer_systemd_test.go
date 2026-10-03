//go:build linux && !nosystemd

package caddy

import (
	"os"
	"reflect"
	"strconv"
	"testing"
)

// TestGetSdListenFd tests the getSdListenFd function for systemd socket activation.
func TestGetSdListenFd(t *testing.T) {
	// Save original environment
	originalFdNames := os.Getenv("LISTEN_FDNAMES")
	originalFds := os.Getenv("LISTEN_FDS")
	originalPid := os.Getenv("LISTEN_PID")

	// Restore environment after test
	defer func() {
		if originalFdNames != "" {
			os.Setenv("LISTEN_FDNAMES", originalFdNames)
		} else {
			os.Unsetenv("LISTEN_FDNAMES")
		}
		if originalFds != "" {
			os.Setenv("LISTEN_FDS", originalFds)
		} else {
			os.Unsetenv("LISTEN_FDS")
		}
		if originalPid != "" {
			os.Setenv("LISTEN_PID", originalPid)
		} else {
			os.Unsetenv("LISTEN_PID")
		}
	}()

	tests := []struct {
		name        string
		fdNames     string
		fds         string
		socketName  string
		expectedFd  uint
		expectError bool
	}{
		{
			name:       "simple http socket",
			fdNames:    "http",
			fds:        "1",
			socketName: "http",
			expectedFd: 3,
		},
		{
			name:       "multiple different sockets - first",
			fdNames:    "http:https:dns",
			fds:        "3",
			socketName: "http",
			expectedFd: 3,
		},
		{
			name:       "multiple different sockets - second",
			fdNames:    "http:https:dns",
			fds:        "3",
			socketName: "https",
			expectedFd: 4,
		},
		{
			name:       "multiple different sockets - third",
			fdNames:    "http:https:dns",
			fds:        "3",
			socketName: "dns",
			expectedFd: 5,
		},
		{
			name:       "duplicate names - first occurrence (no index)",
			fdNames:    "web:web:api",
			fds:        "3",
			socketName: "web",
			expectedFd: 3,
		},
		{
			name:       "duplicate names - first occurrence (explicit index 0)",
			fdNames:    "web:web:api",
			fds:        "3",
			socketName: "web:0",
			expectedFd: 3,
		},
		{
			name:       "duplicate names - second occurrence (index 1)",
			fdNames:    "web:web:api",
			fds:        "3",
			socketName: "web:1",
			expectedFd: 4,
		},
		{
			name:       "complex duplicates - first api",
			fdNames:    "web:api:web:api:dns",
			fds:        "5",
			socketName: "api:0",
			expectedFd: 4,
		},
		{
			name:       "complex duplicates - second api",
			fdNames:    "web:api:web:api:dns",
			fds:        "5",
			socketName: "api:1",
			expectedFd: 6,
		},
		{
			name:       "complex duplicates - first web",
			fdNames:    "web:api:web:api:dns",
			fds:        "5",
			socketName: "web:0",
			expectedFd: 3,
		},
		{
			name:       "complex duplicates - second web",
			fdNames:    "web:api:web:api:dns",
			fds:        "5",
			socketName: "web:1",
			expectedFd: 5,
		},
		{
			name:        "socket not found",
			fdNames:     "http:https",
			fds:         "2",
			socketName:  "missing",
			expectError: true,
		},
		{
			name:        "empty socket name",
			fdNames:     "http",
			fds:         "1",
			socketName:  "",
			expectError: true,
		},
		{
			name:        "missing LISTEN_FDNAMES",
			fdNames:     "",
			fds:         "",
			socketName:  "http",
			expectError: true,
		},
		{
			name:        "index out of range",
			fdNames:     "web:web",
			fds:         "2",
			socketName:  "web:2",
			expectError: true,
		},
		{
			name:        "negative index",
			fdNames:     "web",
			fds:         "1",
			socketName:  "web:-1",
			expectError: true,
		},
		{
			name:        "invalid index format",
			fdNames:     "web",
			fds:         "1",
			socketName:  "web:abc",
			expectError: true,
		},
		{
			name:        "too many colons",
			fdNames:     "web",
			fds:         "1",
			socketName:  "web:0:extra",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment
			if tc.fdNames != "" {
				os.Setenv("LISTEN_FDNAMES", tc.fdNames)
			} else {
				os.Unsetenv("LISTEN_FDNAMES")
			}

			if tc.fds != "" {
				os.Setenv("LISTEN_FDS", tc.fds)
			} else {
				os.Unsetenv("LISTEN_FDS")
			}

			os.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))

			// Test the function
			var (
				listenFdsWithNames map[string][]uint
				err                error
				fd                 uint
			)
			listenFdsWithNames, err = sdListenFdsWithNames()
			if err == nil {
				fd, err = getSdListenFd(listenFdsWithNames, tc.socketName)
			}

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if fd != tc.expectedFd {
					t.Errorf("Expected FD %d but got %d", tc.expectedFd, fd)
				}
			}
		})
	}
}

// TestParseSystemdListenPlaceholder tests that a {systemd.listen.name} placeholder in a listener
// address survives parsing untouched, as the host, and resolves through the replacer to the file
// descriptor systemd passed under that name. The Caddyfile adapter and the server parse listener
// addresses before any placeholder is replaced (caddyconfig/httpcaddyfile/addresses.go,
// modules/caddyhttp/server.go), so the parser has to carry the placeholder through; the http app
// replaces it with ReplaceOrErr before it listens (modules/caddyhttp/app.go), so the replaced
// address has to parse as a plain fd/N or fdgram/N address.
func TestParseSystemdListenPlaceholder(t *testing.T) {
	// Save and restore environment
	originalFdNames := os.Getenv("LISTEN_FDNAMES")
	originalFds := os.Getenv("LISTEN_FDS")
	originalPid := os.Getenv("LISTEN_PID")

	defer func() {
		if originalFdNames != "" {
			os.Setenv("LISTEN_FDNAMES", originalFdNames)
		} else {
			os.Unsetenv("LISTEN_FDNAMES")
		}
		if originalFds != "" {
			os.Setenv("LISTEN_FDS", originalFds)
		} else {
			os.Unsetenv("LISTEN_FDS")
		}
		if originalPid != "" {
			os.Setenv("LISTEN_PID", originalPid)
		} else {
			os.Unsetenv("LISTEN_PID")
		}
	}()

	// Set up test environment
	os.Setenv("LISTEN_FDNAMES", "http:https:dns")
	os.Setenv("LISTEN_FDS", "3")
	os.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))

	// systemd sets LISTEN_* before the process starts, so the provider reads them once at init;
	// this test sets them afterwards, so it re-reads them here and restores the init-time reading.
	savedNameToFiles, savedErr := initNameToFiles, initNameToFilesErr
	defer func() { initNameToFiles, initNameToFilesErr = savedNameToFiles, savedErr }()
	initNameToFiles, initNameToFilesErr = sdListenFdsWithNames()
	if initNameToFilesErr != nil {
		t.Fatalf("reading LISTEN_FDNAMES: %v", initNameToFilesErr)
	}

	repl := NewReplacer()

	tests := []struct {
		input        string
		expectedAddr NetworkAddress // parsed before replacement: the placeholder is the host
		expectedFd   uint           // the host once the placeholder is replaced
		expectErr    bool           // the replacement fails: systemd passed no such socket
	}{
		{
			input: "fd/{systemd.listen.http}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.http}",
			},
			expectedFd: 3,
		},
		{
			input: "fd/{systemd.listen.https}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.https}",
			},
			expectedFd: 4,
		},
		{
			input: "fd/{systemd.listen.dns}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.dns}",
			},
			expectedFd: 5,
		},
		{
			input: "fd/{systemd.listen.http:0}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.http:0}",
			},
			expectedFd: 3,
		},
		{
			input: "fd/{systemd.listen.https:0}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.https:0}",
			},
			expectedFd: 4,
		},
		{
			input: "fdgram/{systemd.listen.http}",
			expectedAddr: NetworkAddress{
				Network: "fdgram",
				Host:    "{systemd.listen.http}",
			},
			expectedFd: 3,
		},
		{
			input: "fdgram/{systemd.listen.https}",
			expectedAddr: NetworkAddress{
				Network: "fdgram",
				Host:    "{systemd.listen.https}",
			},
			expectedFd: 4,
		},
		{
			input: "fdgram/{systemd.listen.http:0}",
			expectedAddr: NetworkAddress{
				Network: "fdgram",
				Host:    "{systemd.listen.http:0}",
			},
			expectedFd: 3,
		},
		{
			input: "fd/{systemd.listen.nonexistent}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.nonexistent}",
			},
			expectErr: true,
		},
		{
			input: "fdgram/{systemd.listen.nonexistent}",
			expectedAddr: NetworkAddress{
				Network: "fdgram",
				Host:    "{systemd.listen.nonexistent}",
			},
			expectErr: true,
		},
		{
			input: "fd/{systemd.listen.http:99}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.http:99}",
			},
			expectErr: true,
		},
		{
			input: "fd/{systemd.listen.invalid:abc}",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "{systemd.listen.invalid:abc}",
			},
			expectErr: true,
		},
		// Test that old fd/N syntax still works
		{
			input: "fd/7",
			expectedAddr: NetworkAddress{
				Network: "fd",
				Host:    "7",
			},
			expectedFd: 7,
		},
		{
			input: "fdgram/8",
			expectedAddr: NetworkAddress{
				Network: "fdgram",
				Host:    "8",
			},
			expectedFd: 8,
		},
	}

	for i, tc := range tests {
		// Before replacement the parser carries the placeholder through as the host.
		actualAddr, err := ParseNetworkAddress(tc.input)
		if err != nil {
			t.Errorf("Test %d (%s): Expected no error but got: %v", i, tc.input, err)
			continue
		}
		if !reflect.DeepEqual(tc.expectedAddr, actualAddr) {
			t.Errorf("Test %d (%s): Expected %+v but got %+v", i, tc.input, tc.expectedAddr, actualAddr)
		}

		// After replacement the host is the descriptor; a placeholder that names no socket is
		// unknown to the replacer, which is an error here as it is for a listener address in the
		// http app.
		replaced, err := repl.ReplaceOrErr(tc.input, true, true)
		if tc.expectErr {
			if err == nil {
				t.Errorf("Test %d (%s): Expected error but got none", i, tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("Test %d (%s): Expected no error but got: %v", i, tc.input, err)
			continue
		}
		replacedAddr, err := ParseNetworkAddress(replaced)
		if err != nil {
			t.Errorf("Test %d (%s): Expected no error but got: %v", i, replaced, err)
			continue
		}
		fd, err := strconv.ParseUint(replacedAddr.Host, 0, strconv.IntSize)
		if err != nil {
			t.Errorf("Test %d (%s): Expected a file descriptor but got %q: %v", i, tc.input, replacedAddr.Host, err)
			continue
		}
		if uint(fd) != tc.expectedFd {
			t.Errorf("Test %d (%s): Expected FD %d but got %d", i, tc.input, tc.expectedFd, fd)
		}
	}
}
