package iron

import (
	"strings"
	"testing"
)

type testModule struct {
	name      string
	score     float64
	detect    bool
	encoded   string
	encodeErr error
	decodeErr error
}

func (m testModule) Name() string {
	return m.name
}

func (m testModule) Detect(string) bool {
	return m.detect
}

func (m testModule) Encode(input string) (string, error) {
	if m.encodeErr != nil {
		return "", m.encodeErr
	}
	if m.encoded == "" {
		return "encoded:" + input, nil
	}
	return m.encoded, nil
}

func (m testModule) Decode(output string) (string, error) {
	if m.decodeErr != nil {
		return "", m.decodeErr
	}
	return "decoded:" + output, nil
}

func (m testModule) Score() float64 {
	return m.score
}

func TestEngine_Process_SelectsHighestScoreModule(t *testing.T) {
	engine := New(
		WithModule(testModule{name: "low", score: 0.1, detect: true}),
		WithModule(testModule{name: "high", score: 0.9, detect: true, encoded: "payload"}),
	)

	output, err := engine.Process("input")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if output != "decoded:payload" {
		t.Fatalf("Process() output = %q, want %q", output, "decoded:payload")
	}
}

func TestEngine_Process_NormalizesInput(t *testing.T) {
	engine := New(WithModule(testModule{name: "trim", score: 1, detect: true}))
	output, err := engine.Process("  hello  ")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if !strings.Contains(output, "hello") {
		t.Fatalf("Process() output = %q, want normalized input", output)
	}
}

func TestEngine_ProcessDetailed_UsesCache(t *testing.T) {
	cache := NewMemoryCache()
	engine := New(
		WithCache(cache),
		WithModule(testModule{name: "cache", score: 1, detect: true}),
	)

	first, err := engine.ProcessDetailed("hello")
	if err != nil {
		t.Fatalf("ProcessDetailed() error = %v", err)
	}
	if first.Cached {
		t.Fatalf("ProcessDetailed() cached = %v, want false", first.Cached)
	}

	second, err := engine.ProcessDetailed("hello")
	if err != nil {
		t.Fatalf("ProcessDetailed() error = %v", err)
	}
	if !second.Cached {
		t.Fatalf("ProcessDetailed() cached = %v, want true", second.Cached)
	}
	if second.Output != first.Output {
		t.Fatalf("ProcessDetailed() output = %q, want %q", second.Output, first.Output)
	}
}

func TestEngine_RegisterModule_ValidatesName(t *testing.T) {
	engine := New()
	if err := engine.RegisterModule(testModule{name: "", detect: true}); err == nil {
		t.Fatalf("RegisterModule() error = nil, want error")
	}
}
