# Unified Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate logging so server and agent write only to service-local application log directories, while frontend does not write file logs.

**Architecture:** Keep application logging in each service runtime (server logger + agent logger). Remove root-level script redirection logs (`./logs/*`) to avoid duplicated/conflicting log planes. Keep frontend on console-only output.

**Tech Stack:** Bash scripts, Go runtime config, markdown docs.

---

### Task 1: Remove root-level dev/prod log redirection

**Files:**
- Modify: `scripts/dev/start-sf.sh`
- Modify: `scripts/agent/start-local.sh`
- Modify: `scripts/server/start.sh`
- Modify: `scripts/agent/start.sh`

**Step 1: Update `scripts/dev/start-sf.sh`**
- Remove `LOG_DIR` and root `logs` creation.
- Keep background launch but redirect stdout/stderr to `/dev/null` so no root log files are created.
- Update startup messages to reflect app-internal logs.

**Step 2: Update `scripts/agent/start-local.sh`**
- Remove root `logs` usage.
- Set env overrides so agent writes to `src/agent/logs/agent-dev.log` via app logger.
- Redirect launcher stdout/stderr to `/dev/null`.

**Step 3: Update `scripts/server/start.sh` and `scripts/agent/start.sh`**
- Remove root `logs` usage.
- Ensure server log dir resolves to `src/server/logs/server`.
- Ensure agent log file resolves to `src/agent/logs/agent.log`.
- Redirect launcher stdout/stderr to `/dev/null`.

### Task 2: Unify runtime defaults

**Files:**
- Modify: `src/server/.env`
- Modify: `src/agent/config/base.yaml`
- Modify: `src/agent/config/dev.yaml`

**Step 1: Server env defaults**
- Set `SERVER_LOG_DIR` to explicit service-local path (`./logs/server` remains valid from `src/server` run path).
- Ensure script-run path uses absolute override when needed.

**Step 2: Agent config defaults**
- Use log file names under a `logs/` subdir (`logs/agent.log`, `logs/agent-dev.log`) so `DataDir + LogFilePath` lands in `src/agent/logs/*` when scripts set `AGENT_DATA_DIR=src/agent`.

### Task 3: Documentation and verification

**Files:**
- Modify: `docs/local-dev.md`
- Modify: `README.md` (if log paths are mentioned)

**Step 1: Document logging behavior**
- Root `logs/` no longer used for service logs.
- Server logs in `src/server/logs/server/*`.
- Agent logs in `src/agent/logs/*.log`.
- Frontend no file logs.

**Step 2: Verification commands**
- Run syntax checks on modified scripts (`bash -n ...`).
- Run grep checks proving root log redirections removed.
- Confirm path mentions in docs are updated.
