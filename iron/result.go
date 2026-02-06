package iron

// Result captures the encoded and decoded representations.
type Result struct {
	Module string
	Input  string
	IR     string
	Output string
	Score  float64
	Cached bool
}
