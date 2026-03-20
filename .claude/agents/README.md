# ShaConnects — Virtual AI Company

> A virtual IT company staffed entirely by AI agents. Every project is delivered end-to-end — from CEO vision to live deployment — by a structured team of specialist agents, each with elite-level skill in their domain.

---

## Agent Roster (15 agents)

### Leadership
| File | Role | Model | Reports To |
|------|------|-------|-----------|
| `cto.md` | CTO | opus · auto | CEO (You) |
| `project-manager.md` | Project Manager | sonnet · plan | CTO |

### Product
| File | Role | Model | Reports To |
|------|------|-------|-----------|
| `product-manager.md` | Product Manager | sonnet · plan | CTO |
| `ui-ux-designer.md` | UI/UX Designer | sonnet · plan | Product Manager |

### Engineering
| File | Role | Model | Reports To |
|------|------|-------|-----------|
| `tech-lead.md` | Tech Lead | sonnet · plan | CTO |
| `backend-developer.md` | Backend Developer | sonnet · auto | Tech Lead |
| `frontend-developer.md` | Frontend Developer | sonnet · auto | Tech Lead |
| `mobile-developer.md` | Mobile Developer | sonnet · auto | Tech Lead |
| `database-admin.md` | Database Admin | sonnet · plan | Tech Lead |
| `infra-engineer.md` | Infra Engineer | sonnet · auto | Tech Lead |

### Quality & Security
| File | Role | Model | Reports To |
|------|------|-------|-----------|
| `qa-engineer.md` | QA Engineer | sonnet · plan | CTO |
| `code-reviewer.md` | Code Reviewer | sonnet · plan | QA Engineer |
| `security-auditor.md` | Security Auditor | sonnet · plan | QA Engineer |
| `performance-engineer.md` | Performance Engineer | sonnet · plan | QA Engineer |

### Delivery
| File | Role | Model | Reports To |
|------|------|-------|-----------|
| `devops-engineer.md` | DevOps Engineer | sonnet · auto | CTO |
| `debugger.md` | Debugger | sonnet · plan | DevOps Engineer |
| `tech-writer.md` | Tech Writer | sonnet · auto | DevOps Engineer |

---

## Agent Template

Every agent follows this structure:

```markdown
---
name: role-name-kebab-case
description: >
  Trigger conditions — when Claude invokes this agent.
  Be specific: actions, file types, phases.
tools: Read, Edit, Write, Grep, Glob, Bash   ← only what the role needs
model: sonnet | opus | inherit
permissionMode: auto | plan
---

You are a [Seniority] [Role] at **ShaConnects**...

## Identity & Scope
## Tech Stack & Environment
## Core Responsibilities
## Standards & Conventions
## Workflow
## Output Format
```

---

## Project Lifecycle

```
CEO kicks off project
    │
    ▼
Product Manager  →  PRD + user stories + acceptance criteria
    │
    ▼
CTO              →  Architecture Decision Record (ADR)
    │
    ▼
Tech Lead        →  Engineering breakdown + API contracts
    │
    ├── Backend Developer   →  Go API + WebSocket + queue
    ├── Frontend Developer  →  React + TypeScript + Zustand
    ├── Mobile Developer    →  React Native iOS + Android
    ├── Database Admin      →  Schema + migrations + indexes
    └── Infra Engineer      →  Dockerfiles + K8s + CI/CD
    │
    ▼
UI/UX Designer   →  Screen specs + design system + a11y
    │
    ▼
QA Engineer gate:
    ├── Code Reviewer       →  Quality + patterns + coverage
    ├── Security Auditor    →  CVEs + OWASP + auth + secrets
    └── Performance Eng     →  Load test + profiling + budgets
    │
    ▼
DevOps Engineer  →  Cloud deploy + monitoring + alerting
    │
    ▼
Tech Writer      →  README + API docs + runbooks
Debugger         →  Production issue triage (on demand)
    │
    ▼
🟢 LIVE
```

---

## Origin of Agents

| Agent | Origin |
|-------|--------|
| `backend-developer` | Split from `fullstack-developer.md` (Role 1) |
| `frontend-developer` | Split from `fullstack-developer.md` (Role 2) |
| `code-reviewer` | Split from `code-security-reviewer.md` (Role 1) |
| `security-auditor` | Split from `code-security-reviewer.md` (Role 2) |
| `infra-engineer` | Split from `infra-debugger.md` (Role 1) |
| `debugger` | Split from `infra-debugger.md` (Role 2) |
| All others | New agents |

---

## Usage with Claude Code

Place all `.md` files in your project's `.claude/agents/` directory.
Claude Code reads the `description` field of each agent to decide when to invoke it automatically.

```
your-project/
└── .claude/
    └── agents/
        ├── cto.md
        ├── project-manager.md
        ├── product-manager.md
        ├── ui-ux-designer.md
        ├── tech-lead.md
        ├── backend-developer.md
        ├── frontend-developer.md
        ├── mobile-developer.md
        ├── database-admin.md
        ├── infra-engineer.md
        ├── qa-engineer.md
        ├── code-reviewer.md
        ├── security-auditor.md
        ├── performance-engineer.md
        ├── devops-engineer.md
        ├── debugger.md
        └── tech-writer.md
```
