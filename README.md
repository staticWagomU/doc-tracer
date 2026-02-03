# doc-tracer

**Bidirectional traceability between documentation and code**

[English](README.md) | [日本語](README.ja.md)

<img width="1263" height="738" alt="Screenshot" src="https://github.com/user-attachments/assets/160b1347-790c-49a4-b1b7-f0a1e7ee6b27" />

---

## The Problem This Tool Solves

### Problem 1: Documentation-Code Drift

In software development, documentation and code inevitably diverge.

- Code changes made, but related documents weren't updated
- Unclear which documents need updating
- Documentation becomes stale and loses credibility

### Problem 2: Invisible Impact Scope

When modifying code, it's difficult to grasp the full impact:

- Which screens are affected when I change this API?
- Which spec documents need updating when I modify this component?
- Which code needs changing when requirements change?

### Problem 3: AI Documentation Generation Limits

When AI tools like Claude Code generate/modify code, updating related documentation remains challenging:

- AI understands code context but not the entire documentation structure
- Cannot determine which documents relate to "this code"
- Documentation updates get missed

---

## doc-tracer's Approach

### Core Idea: Explicit Relationships via Graph

doc-tracer represents documentation-code relationships as a **directed graph**.

<img width="1024" height="559" alt="image" src="https://github.com/user-attachments/assets/2cd74431-015d-4096-bd66-b63dca21ac69" />

**Nodes**: Documents, components, APIs, DB tables, etc.
**Edges**: contains (parent-child), implements, uses, calls

### Two Information Sources

1. **YAML Frontmatter (Manual)**: Explicitly define document hierarchy and code references
2. **Static Analysis (Automatic)**: Auto-detect imports, API calls, template usage in code

This combination enables **Documentation → Code → Documentation** bidirectional tracing.

---

## Installation

### go install (Recommended)

```bash
go install github.com/tomohiro-owada/doc-tracer@latest
```

### Download Binary

Download from [Releases](https://github.com/tomohiro-owada/doc-tracer/releases) for your platform:
- macOS (Intel / Apple Silicon)
- Linux (amd64 / arm64)
- Windows (amd64)

### Build from Source

```bash
git clone https://github.com/tomohiro-owada/doc-tracer.git
cd doc-tracer
go build -o doc-tracer .
```

---

## Quick Start

### 1. Add YAML Frontmatter to Documents

```yaml
---
trace:
  id: doc:docs/03_design/auth-design.md
  parent:
    - doc:docs/02_spec/auth-spec.md
  uses:
    - component:LoginForm
    - api:/api/v1/auth/login
---

# Authentication Design Document
...
```

### 2. Scan Your Project

```bash
doc-tracer scan .
```

### 3. Check Impact of Code Changes

```bash
doc-tracer impact src/components/LoginForm.vue
```

Output:
```
Impacted documents (5):

  - LoginForm-design.md
    Path: docs/05_program/LoginForm-design.md

  - login-detail.md
    Path: docs/04_detail/login-detail.md

  - auth-basic.md
    Path: docs/03_basic/auth-basic.md

  ...
```

### 4. Verify Consistency

```bash
doc-tracer check
```

### 5. Visualize (Optional)

```bash
doc-tracer serve --port 8080
```

Open http://localhost:8080 in your browser.

---

## Supported Languages

doc-tracer automatically scans and analyzes these languages:

| Language | Extensions | Detection Targets |
|----------|------------|-------------------|
| **JavaScript/TypeScript** | .js, .ts, .tsx | modules, composables, stores |
| **Vue** | .vue | components, API calls |
| **PHP** | .php | controllers, models, routes |
| **Go** | .go | modules, functions, methods |
| **Python** | .py | classes, functions, Flask/FastAPI routes |
| **Java** | .java | classes, methods, Spring Boot endpoints |
| **Rust** | .rs | structs, functions, Actix-web/Axum routes |
| **Ruby** | .rb | classes, methods |
| **C#** | .cs | classes, methods, ASP.NET endpoints |
| **Kotlin** | .kt, .kts | classes, functions, Spring Boot endpoints |
| **Swift** | .swift | classes, structs, functions |
| **Dart** | .dart | classes, widgets, screens, Provider/Bloc |

Each parser automatically determines appropriate node types (controller, model, service, view, etc.) based on directory structure.

---

## 5-Layer Document Structure

doc-tracer assumes a waterfall-style 5-layer document structure:

```
01_requirements/     # WHY - Why are we building this?
02_specifications/   # WHAT - What are we building?
03_basic_design/     # HOW (Overview) - How will it work?
04_detailed_design/  # HOW (Detail) - Detailed design
05_program_design/   # Implementation design → Code references
```

### Layer Relationships

| Relation | Meaning | Example |
|----------|---------|---------|
| `contains` | Parent → Child (hierarchy) | Basic design → Detailed design |
| `implements` | Lower → Upper (implementation) | Program design → Spec |
| `uses` | Reference | Design doc → Component |

---

## YAML Frontmatter Specification

```yaml
---
trace:
  # Document ID (auto-generated from path if omitted)
  id: doc:docs/03_basic_design/auth-design.md

  # Parent documents (upper layer that contains this doc)
  parent:
    - doc:docs/02_spec/auth-spec.md

  # Child documents (lower layer derived from this doc)
  children:
    - doc:docs/04_detail/login-detail.md

  # Implementation target (upper requirement this doc implements)
  implements:
    - doc:docs/01_requirements/functional-requirements.md

  # Code elements this document references
  uses:
    - component:LoginForm
    - api:/api/v1/auth/login
    - module:auth
---
```

### Node ID Prefixes

| Prefix | Target | Auto/Manual |
|--------|--------|-------------|
| `doc:` | Document | Auto (from path) |
| `component:` | Vue component | Auto |
| `store:` | Pinia store | Auto |
| `composable:` | Vue Composable | Auto |
| `api:` | API endpoint | Auto/Manual |
| `controller:` | Controller | Auto |
| `model:` | Model | Auto |
| `module:` | Module | Auto |
| `class:` | Class | Auto |
| `func:` | Function | Auto |

---

## Command Reference

### scan

Scan documents and code to build the traceability graph.

```bash
doc-tracer scan [path] [flags]

Flags:
  --docs-only    Scan documents only
  --code-only    Scan code only
  --db string    SQLite database path (default "tracer.db")
```

### impact

Find documents impacted by a file change.

```bash
doc-tracer impact <file> [flags]

Flags:
  --json         Output as JSON
  --db string    SQLite database path (default "tracer.db")
```

### check

Check consistency (broken links, orphaned documents).

```bash
doc-tracer check [flags]

Flags:
  --db string    SQLite database path (default "tracer.db")
```

### serve

Start the visualization web UI.

```bash
doc-tracer serve [flags]

Flags:
  --port int     Port number (default 8080)
  --db string    SQLite database path (default "tracer.db")
```

---

## Integration with Claude Code

### Workflow

```bash
# 1. After code changes, check impact
doc-tracer impact src/components/LoginForm.vue

# 2. Update impacted documents (ask Claude Code)
# "Please update the following documents: ..."

# 3. Verify consistency
doc-tracer check

# 4. Commit
git add . && git commit -m "feat: improve login feature"
```

---

## Database

Uses SQLite for persistence. Default file: `tracer.db`

### Schema

**nodes**: Nodes (documents, code elements)
```sql
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    file_path TEXT,
    file_hash TEXT,
    updated_at DATETIME
);
```

**edges**: Edges (relationships)
```sql
CREATE TABLE edges (
    from_id TEXT NOT NULL,
    to_id TEXT NOT NULL,
    relation TEXT NOT NULL,
    source TEXT DEFAULT 'auto',
    defined_in TEXT,
    UNIQUE(from_id, to_id, relation)
);
```

---

## Roadmap

- [x] Git diff integration (`doc-tracer impact --staged`)
- [x] Claude Code skill integration
- [x] More language support (13 languages)

---

## License

MIT License

---

## Contributing

Issues and Pull Requests are welcome!

https://github.com/tomohiro-owada/doc-tracer
