package driver

import "testing"

func TestRegisterandGet(t *testing.T) {
	called := false
	Register("test-engine", func(cfg ConnectionConfig) (Connector, Backuper, Restorer) {
			called = true
			return nil, nil, nil
	})
	f, ok := Get("test-engine")
	if !ok {
		t.Fatal("expected test-engine to be registered")
	}
	f(ConnectionConfig{})
	if !called {
		t.Error("expected function to be called")
	}
}

func TestGet_UnknownEngine(t *testing.T) {
	if _, ok := Get("unknown-engine"); ok {
		t.Error("expected ok=false for unrefistered engine")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	Register("dup-engine", func(cfg ConnectionConfig) (Connector, Backuper, Restorer) { return nil, nil, nil })
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic on duplicate registration")
		}
	}()
	Register("dup-engine", func(cfg ConnectionConfig) (Connector, Backuper, Restorer) { return nil, nil, nil })
}