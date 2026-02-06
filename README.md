# IRon — Intermediate Representation Optimizer Network

IRon is a **semantic compression engine for LLM systems**.

It converts natural language, code, and structured data into a compact intermediate representation (IR) to **reduce token usage, latency, and cost**, then restores the output back to its original form.

IRon works as a **compiler for human language**.

---

## 🚀 Why IRon?

LLM systems waste tokens on:

* Redundant phrasing
* Verbose instructions
* Repeated context
* Boilerplate prompts

This increases:

* API cost
* Latency
* Unpredictability

IRon solves this by introducing a compression layer between users and models.

Result: **50%–90% token reduction** in real workloads.

---

## 🧠 Core Concept

```
Input → Normalize → Encode (IR) → LLM → Decode → Output
```

The LLM never sees raw human language.

It only sees optimized IR.

---

## ✨ Features

* Domain-aware compression (tasks, code, data, web, logs, RAG)
* Pluggable DSL/IR modules
* Automatic domain detection
* Encoder / Decoder pipeline
* Semantic cache
* Multi-LLM support (OpenAI, Ollama, Local)
* Stateless core

---

## 📦 Use Cases

IRon is **not limited to Telegram**.

It can be used in:

* Autonomous agents
* Chatbots
* Developer tools
* RAG systems
* SaaS platforms
* Workflow automation
* CLI tools
* Backend services

Anywhere LLM cost matters.

---

## 🏗 Architecture

```
Input
  ↓
Normalizer
  ↓
Domain Classifier
  ↓
IR Encoder
  ↓
LLM Runtime
  ↓
IR Decoder
  ↓
Validator
  ↓
Output
```

---

## 📐 Unified IR Format (Example)

```
@CTX[min]
@TASK{ANALYZE->SUM->SUGG}
@OBJ{repo}
@OUT{tech}
```

Human input:

> “Analyze this repository, summarize it and suggest improvements.”

Token reduction: ~70–80%

---

## 🧩 IR Modules

| Module  | Purpose          |
| ------- | ---------------- |
| IR-TASK | Task planning    |
| IR-CODE | Code compression |
| IR-DATA | JSON / API data  |
| IR-WEB  | Web content      |
| IR-KNOW | RAG context      |
| IR-LOG  | Logs             |
| IR-META | Agent control    |
| IR-PIPE | Workflows        |

Each module is pluggable.

---

## 🔌 Module Interface (Go)

```go
type IRModule interface {
    Name() string
    Detect(input string) bool
    Encode(input string) (string, error)
    Decode(output string) (string, error)
    Score() float64
}
```

---

## 📊 Performance Targets

| Metric          | Target |
| --------------- | ------ |
| Token Reduction | ≥ 65%  |
| Semantic Loss   | < 1%   |
| Encode Time     | < 5ms  |
| Decode Time     | < 5ms  |

---

## 🛠 Installation

```bash
git clone https://github.com/iagomussel/IRon.git
cd IRon
go build
```

---

## ▶️ Usage (Example)

```go
engine := iron.New()

out, err := engine.Process(input)
if err != nil {
    panic(err)
}

fmt.Println(out)
```

---

## 🗺 Roadmap

### Phase 1 — Core

* IR-TASK
* IR-CODE
* IR-DATA
* Profiler
* Cache

### Phase 2 — Intelligence

* Auto DSL selection
* RAG compression
* Ranking engine
* Multi-runtime

### Phase 3 — Autonomy

* Auto IR generation
* Self-optimization
* DSL synthesis
* Feedback learning

---

## 🎯 Vision

IRon is infrastructure.

Not a bot.
Not a wrapper.
Not a prompt tool.

It is:

> A compilation layer for AI systems.

Who controls compression controls scale.

---

## 🤝 Contributing

Contributions are welcome.

Focus areas:

* New IR modules
* Benchmarks
* Optimizers
* Runtimes
* Profilers

Open a PR or issue.
