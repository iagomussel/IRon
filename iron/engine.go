package iron

import (
	"errors"
	"strings"
)

// Normalizer prepares input for module selection and encoding.
type Normalizer func(input string) string

// Engine coordinates normalization, module selection, encoding, and decoding.
type Engine struct {
	modules     []IRModule
	normalizers []Normalizer
	cache       Cache
}

// Option configures the Engine.
type Option func(*Engine)

// WithModule registers a module with the engine.
func WithModule(module IRModule) Option {
	return func(e *Engine) {
		if module == nil {
			return
		}
		if err := e.RegisterModule(module); err != nil {
			return
		}
	}
}

// WithNormalizer registers an additional normalizer.
func WithNormalizer(normalizer Normalizer) Option {
	return func(e *Engine) {
		if normalizer == nil {
			return
		}
		e.normalizers = append(e.normalizers, normalizer)
	}
}

// WithCache enables caching for processed inputs.
func WithCache(cache Cache) Option {
	return func(e *Engine) {
		e.cache = cache
	}
}

// New creates a new Engine with a passthrough module by default.
func New(options ...Option) *Engine {
	e := &Engine{
		modules:     []IRModule{PassthroughModule{}},
		normalizers: []Normalizer{strings.TrimSpace},
	}
	for _, option := range options {
		option(e)
	}
	return e
}

// RegisterModule registers a module with validation.
func (e *Engine) RegisterModule(module IRModule) error {
	if module == nil {
		return ErrNilModule
	}
	if strings.TrimSpace(module.Name()) == "" {
		return ErrEmptyModuleName
	}
	e.modules = append(e.modules, module)
	return nil
}

// Modules returns the registered modules in order.
func (e *Engine) Modules() []IRModule {
	modules := make([]IRModule, len(e.modules))
	copy(modules, e.modules)
	return modules
}

// Process normalizes the input, selects the best module, encodes, then decodes.
func (e *Engine) Process(input string) (string, error) {
	result, err := e.ProcessDetailed(input)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

// ProcessDetailed returns the IR and output with metadata.
func (e *Engine) ProcessDetailed(input string) (Result, error) {
	normalized := e.normalize(input)
	if e.cache != nil {
		if cached, ok := e.cache.Get(normalized); ok {
			cached.Cached = true
			return cached, nil
		}
	}

	module := e.selectModule(normalized)
	if module == nil {
		result := Result{Input: normalized, Output: normalized}
		if e.cache != nil {
			e.cache.Set(normalized, result)
		}
		return result, nil
	}

	encoded, err := module.Encode(normalized)
	if err != nil {
		return Result{}, err
	}
	decoded, err := module.Decode(encoded)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Module: module.Name(),
		Input:  normalized,
		IR:     encoded,
		Output: decoded,
		Score:  module.Score(),
	}
	if e.cache != nil {
		e.cache.Set(normalized, result)
	}
	return result, nil
}

func (e *Engine) normalize(input string) string {
	value := input
	for _, normalizer := range e.normalizers {
		value = normalizer(value)
	}
	return value
}

func (e *Engine) selectModule(input string) IRModule {
	var (
		bestModule IRModule
		bestScore  float64
	)
	for _, module := range e.modules {
		if module == nil {
			continue
		}
		if !module.Detect(input) {
			continue
		}
		score := module.Score()
		if bestModule == nil || score > bestScore {
			bestModule = module
			bestScore = score
		}
	}
	return bestModule
}

var (
	ErrEmptyModuleName = errors.New("module name cannot be empty")
	ErrNilModule       = errors.New("module cannot be nil")
)

// PassthroughModule is the default module that preserves input.
type PassthroughModule struct{}

func (PassthroughModule) Name() string {
	return "IR-PASS"
}

func (PassthroughModule) Detect(string) bool {
	return true
}

func (PassthroughModule) Encode(input string) (string, error) {
	return input, nil
}

func (PassthroughModule) Decode(output string) (string, error) {
	return output, nil
}

func (PassthroughModule) Score() float64 {
	return 0
}
