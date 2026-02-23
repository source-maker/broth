# Architecture Decision Records (ADR)

This directory contains Architecture Decision Records for the Broth framework.

## What is an ADR?

An ADR captures an important architectural decision along with its context and consequences. Each ADR describes a decision, the factors that led to it, and the expected impact.

## ADR Workflow

```
Proposal → GitHub Discussion → Consensus → ADR file → Implementation PR
                                  │
                                  └─ Rejected → Discussion closed (with rationale)
```

### 1. Propose via GitHub Discussion

Open a new discussion in the **"Architecture Proposals"** category with:

- **Title**: Short description of the decision
- **Context**: What problem are you solving?
- **Options considered**: At least 2 alternatives with pros/cons
- **Recommendation**: Your preferred option and rationale

### 2. Discuss and reach consensus

The community and maintainers discuss the proposal. Maintainers will label the discussion as:
- `accepted` — Ready to be formalized as an ADR
- `rejected` — Closed with rationale documented
- `deferred` — Postponed to a future phase

### 3. Formalize as an ADR

Once accepted, create an ADR file in this directory:

- **Filename**: `NNNN-short-title.md` (zero-padded sequence number)
- **Format**: See [ADR template](#template) below
- **PR**: Submit a PR that includes the ADR file (link to the original Discussion)

### 4. Link to implementation

The implementation PR should reference the ADR file and the original Discussion.

## File Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-bob-over-sqlc-and-gorm.md) | Bob をデータアクセスライブラリとして採用 | Accepted |
| [0002](0002-apps-design-proposal.md) | Django apps 概念の段階的導入設計 | Proposed (deferred) |
| [0003](0003-competitive-analysis-positioning.md) | 競合分析と Broth のポジショニング | Accepted |
| [0004](0004-design-review-actions.md) | 設計レビュー指摘と対応方針 | Accepted |

For the full list of 37 ADRs (including those embedded in design documents), see [DECISION_LOG.md](../DECISION_LOG.md).

## Template

```markdown
# ADR-NNNN: Title

- **Status**: Proposed | Accepted | Deprecated | Superseded by ADR-XXXX
- **Date**: YYYY-MM-DD
- **Discussion**: (link to GitHub Discussion)

## Context

What is the issue that we're seeing that is motivating this decision?

## Options Considered

### Option A
Description, pros, cons.

### Option B
Description, pros, cons.

## Decision

What is the change that we're proposing and/or doing?

## Consequences

What becomes easier or more difficult to do because of this change?
```
