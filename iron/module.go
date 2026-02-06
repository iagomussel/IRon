package iron

// IRModule defines the contract for domain-specific encoding/decoding.
type IRModule interface {
	Name() string
	Detect(input string) bool
	Encode(input string) (string, error)
	Decode(output string) (string, error)
	Score() float64
}
