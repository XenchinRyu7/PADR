PRD — Personal Autonomous Development Runner
Codename: PADR
Nama: Personal Autonomous Development Runner
Goal: menjalankan autonomous coding tasks pada beberapa repository secara terjadwal menggunakan AI coding agent lokal/CLI, dengan model/provider swapping dan automatic Git workflow.
1. Problem
User punya beberapa repository yang ingin terus dikembangkan, tetapi:
- sering lupa melanjutkan development;
- tidak selalu punya waktu untuk membuka IDE;
- cloud coding agent bisa mahal;
- model/provider gratis memiliki quota berbeda;
- setiap repository punya roadmap dan konteks berbeda.
PADR menjadi personal development worker yang berjalan di laptop user.
Laptop
  │
  └── PADR
       ├── Scheduler
       ├── Repository Manager
       ├── Agent Runner
       ├── Model Router
       ├── Git Manager
       └── Execution Logger

2. Core Concept
User menentukan:
project: food-erp
repository: C:/dev/food-erp

schedule:
  cron: "0 9 * * *"

agent:
  engine: opencode

models:
  - groq/openai/gpt-oss-120b
  - google/gemini-...
  - openrouter/...

task:
  source: ROADMAP.md
  max_tasks: 2

PADR kemudian:
09:00
 ↓
select project
 ↓
check Git state
 ↓
select available model
 ↓
start OpenCode
 ↓
give task
 ↓
agent edits repository
 ↓
run tests
 ↓
review git diff
 ↓
commit
 ↓
push
 ↓
log result

3. MVP Scope
MVP wajib punya
Scheduler
- daily
- weekly
- custom cron
- multiple projects
- configurable execution window
Repository Manager
- repository path
- branch
- clean/dirty check
- pull before execution
- push after successful execution
Agent Engine
OpenCode CLI sebagai default.
opencode run "..."

OpenCode memang mendukung mode non-interaktif khusus untuk scripting/automation. OpenCode
Model Provider
Minimal:
Groq
Gemini
OpenRouter
Ollama/local

dan desainnya provider-agnostic.
Model Swapping
Contoh:
models:
  - id: groq-fast
    provider: groq
    model: openai/gpt-oss-120b
    api_key: GROQ_API_KEY

  - id: gemini
    provider: google
    model: gemini-...
    api_key: GEMINI_API_KEY

  - id: local
    provider: ollama
    model: qwen...

OpenCode sendiri mendukung provider custom/OpenAI-compatible dan environment-based credentials, jadi PADR tidak perlu mengimplementasikan API client untuk setiap LLM. OpenCode
4. Model Swapping / Fallback
Ini fitur inti.
Jangan:
Groq habis
→ automation mati

Tapi:
              Task
                ↓
        ┌── Model Router ──┐
        ↓                  ↓
      Model A            Model B
        ↓                  ↓
     success             fallback

Contoh policy:
routing:
  strategy: fallback

  models:
    - groq-fast
    - gemini-fast
    - openrouter-free
    - ollama-local

PADR mencoba:
1. Groq
   ↓ rate limit
2. Gemini
   ↓ unavailable
3. OpenRouter
   ↓ unavailable
4. Ollama

Tidak ada auto-charge.
Provider yang tidak punya saldo/kuota jangan digunakan.
5. Budget Guard
Ini WAJIB, karena lu ninggalin laptop tidur dan agent bisa autonomous.
Contoh:
limits:
  max_runs_per_day: 5
  max_runtime_minutes: 45
  max_tasks_per_run: 2
  max_commits_per_run: 5

Dan:
providers:
  groq:
    max_daily_runs: 2

  gemini:
    max_daily_runs: 2

  openrouter:
    max_daily_runs: 1

Nanti PADR tahu:
"Quota provider A habis → jangan dipaksa."

6. Project Definition
Setiap repository punya file:
.padr/project.yaml

Contoh:
name: food-erp

repository:
  path: C:/dev/food-erp
  branch: main

agent:
  engine: opencode

development:
  roadmap: ROADMAP.md
  max_tasks: 2

git:
  auto_pull: true
  auto_commit: true
  auto_push: true

validation:
  commands:
    - ./mvnw test

schedule:
  cron: "0 9 * * *"

Jadi PADR nggak perlu tahu isi project secara hardcoded.
7. Agent Prompt
PADR generate instruction berdasarkan project.
Misalnya:
You are the autonomous developer for this repository.

Read:
- PROJECT.md
- ROADMAP.md
- ARCHITECTURE.md

Rules:
1. Pick the next incomplete task.
2. Do not work on unrelated features.
3. Keep the change small.
4. Run the project's validation commands.
5. Fix failures caused by your changes.
6. Do not modify secrets.
7. Do not rewrite architecture without justification.
8. Commit only after validation passes.
9. Push only the changes created during this run.

Complete at most 2 tasks.

Agent kemudian bekerja lewat OpenCode.
8. Git Safety
Sebelum agent:
git status

Kalau dirty:
STOP

Default jangan menyentuh kerjaan manual user.
Flow:
dirty working tree
       ↓
    STOP RUN
       ↓
log:
"Repository has uncommitted changes"

Kalau clean:
git pull --ff-only
↓
agent
↓
tests
↓
git diff
↓
commit
↓
push

9. Commit Policy
Kita tidak memaksa 5 commit.
Agent mengerjakan actual development.
Target:
development:
  max_tasks: 2
  max_commits: 5

Kalau satu feature menghasilkan:
feat: add pagination
test: add pagination tests
refactor: extract pagination mapper

→ valid.
Kalau cuma menghasilkan satu commit:
feat: implement pagination

→ juga valid.
Contribution graph adalah efek samping, bukan target utama agent.
10. Execution Log
PADR menyimpan:
~/.padr/
├── config/
├── logs/
├── runs/
└── state/

Contoh:
{
  "project": "food-erp",
  "started_at": "2026-10-04T09:00:00",
  "provider": "groq",
  "model": "openai/gpt-oss-120b",
  "status": "success",
  "commits": 2,
  "duration_seconds": 1842
}

Jadi lu bisa melihat:
October 4

Food ERP
✓ 2 commits
✓ tests passed
✓ Groq

Java Toolkit
✓ 1 commit
✓ tests passed
✓ Gemini

RAG Project
✗ skipped
  Groq quota exhausted
  Gemini unavailable
  local fallback disabled

11. Scheduler
Untuk MVP jangan bikin scheduler dari nol dulu.
PADR punya scheduler abstraction:
PADR Scheduler
      ↓
Windows Task Scheduler

Jadi PADR bisa melakukan:
padr install-schedule

yang membuat Windows Scheduled Task.
Misalnya:
09:00 → Food ERP
13:00 → Java Toolkit
18:00 → AI Project
23:00 → Maintenance

Nanti kalau mau cross-platform:
Windows → Task Scheduler
Linux   → systemd timer / cron
macOS   → launchd

12. CLI
CLI-nya gue bayangin:
padr init

padr project add food-erp

padr project list

padr run food-erp

padr run --all

padr status

padr logs

padr models

padr quota

padr schedule install

padr schedule list

13. Model Configuration
Misalnya:
padr provider add groq

lalu:
API Key:
************

PADR jangan menyimpan API key plaintext di repository.
Untuk MVP:
Windows Credential Manager

atau environment variables.
Project config hanya:
provider: groq
model: openai/gpt-oss-120b

OpenCode sendiri punya credential management dan environment/config support, jadi PADR bisa meneruskan konfigurasi tanpa mengurus token model secara langsung. OpenCode
14. Tech Stack
Gue bakal pilih:
PADR
Golang
Kenapa?
- cepat dibuat;
- subprocess gampang;
- YAML/JSON gampang;
- Windows integration gampang;
- CLI ecosystem bagus;
- cocok buat automation.
Stack:
Golang
├── Cobra           CLI framework
├── gopkg.in/yaml.v3 YAML parser & config schema
├── fatih/color & olekukonko/tablewriter terminal UI
├── os/exec         subprocess & agent execution
├── Git CLI wrapper git integration
└── modernc.org/sqlite pure Go SQLite local state

Database: SQLite (pure Go, CGO-free).
Nggak perlu PostgreSQL.
Agent Engine
OpenCode CLI
PADR tidak menjadi AI agent.
PADR cuma:
ORCHESTRATOR
    ↓
OpenCode
    ↓
LLM provider

Ini penting karena kalau besok OpenCode nggak cocok:
OpenCode
   ↓
Cline

kita tinggal implement:
type AgentEngine interface {
    Run(ctx context.Context, req AgentRunRequest) (*AgentRunResult, error)
}

Kemudian:
- OpenCodeEngine
- ClineEngine
- AiderEngine

Cline juga punya headless CLI dan schedule/automation capability, sehingga sangat cocok dijadikan adapter kedua nanti. GitHub
15. Architecture
┌───────────────────────────────────────────┐
│                   PADR                    │
│                                           │
│  ┌───────────┐       ┌────────────────┐  │
│  │ Scheduler │──────▶│ Task Dispatcher│  │
│  └───────────┘       └───────┬────────┘  │
│                              │            │
│                              ▼            │
│                     ┌────────────────┐    │
│                     │ Model Router   │    │
│                     └───────┬────────┘    │
│                             │             │
│                    ┌────────┼────────┐    │
│                    ▼        ▼        ▼    │
│                  Groq    Gemini   Ollama  │
│                                           │
│                             │             │
│                             ▼             │
│                    ┌────────────────┐     │
│                    │ Agent Adapter  │     │
│                    └───────┬────────┘     │
│                            │              │
│                            ▼              │
│                         OpenCode          │
│                            │              │
│                            ▼              │
│                         Git Repo          │
│                            │              │
│                            ▼              │
│                       Test / Lint         │
│                            │              │
│                            ▼              │
│                       Git Commit          │
│                            │              │
│                            ▼              │
│                         Git Push          │
└───────────────────────────────────────────┘

16. Non-Goals MVP
Jangan bikin:
- web dashboard;
- cloud server;
- multi-user;
- remote execution;
- own LLM inference;
- own agent framework;
- Docker orchestration;
- Kubernetes 💀;
- fancy contribution analytics.
PADR adalah personal local daemon/CLI.
17. MVP Milestone
Phase 1 — Agent runner
padr run ./food-erp

→ OpenCode menjalankan task.
Phase 2 — Git safety
pull
→ agent
→ test
→ diff
→ commit
→ push

Phase 3 — Project configuration
.padr/project.yaml

Phase 4 — Model router
Groq
 ↓
Gemini
 ↓
OpenRouter
 ↓
Ollama

Phase 5 — Scheduler
Windows Task Scheduler

Phase 6 — Observability
padr status
padr logs
padr quota