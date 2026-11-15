package libdave

import (
	"runtime"
	"testing"
	"time"
	"weak"
)

func TestMaxSupportedProtocolVersion(t *testing.T) {
	maxSupportedProtocolVersion := MaxSupportedProtocolVersion()

	if maxSupportedProtocolVersion != 1 {
		t.Errorf("expected 1, got %d", maxSupportedProtocolVersion)
	}
}

func TestEncryptorCallbackAfterGC(t *testing.T) {
	encryptor := NewEncryptor()
	runtime.GC()
	godaveProtocolVersionChangedCallback(encryptor.userData)
	encryptor.SetPassthroughMode(true)
	if !encryptor.IsPassthroughMode() {
		t.Fatal("encryptor did not enter passthrough mode")
	}
	runtime.KeepAlive(encryptor)
}

func TestDecryptorCanBeCollected(t *testing.T) {
	decryptor := weak.Make(NewDecryptor())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if decryptor.Value() == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("decryptor remained reachable after garbage collection")
}
