# Spec Memory File Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `spec-memory` first. Keep `.spec` as the explicit file import/export format. Do not silently dual-write.

**Goal:** Add explicit synchronization between tracked project `.spec` files and backend Spec Memory so multiple users can share durable project memory through git and inspect/edit it in Multica.

**Stage 3 Scope:** Implement explicit `.spec` file import/export between git-tracked project memory and backend Spec Memory. Import durable long-term memory plus issue execution state that resolves to existing backend issues. Export backend Spec Memory back to `.spec` with local-file overwrite protection by default.

---

## Boundaries

- `.spec/epics/**` and `.spec/decisions/**` are durable knowledge and can be imported to backend.
- `.spec/issues/**` is issue execution state. Import it only when the local issue id resolves to a backend issue UUID or Multica identifier; otherwise report it as skipped.
- No automatic background sync.
- No sync runs without an explicit `multica spec sync --from-files` or `--to-files`.
- `--to-files` must not overwrite existing local files unless `--force` is provided.
- No issue execution state in module `00-index.md`.

---

### Task 1: File Snapshot Parser

- [x] Add an exported `.spec` file snapshot reader in `server/internal/specmem`.
- [x] Capture epics, modules, known module docs, epic index docs, decisions, issue state files, and skipped issue files.
- [x] Keep unsupported project-level files such as `.spec/index.md` and `glossary.md` out of the first backend import unless a backend scope exists.

### Task 2: Backend Import Endpoint

- [x] Add `POST /api/spec/sync/from-files`.
- [x] Upsert epics/modules/documents/decisions from a snapshot.
- [x] Return counts, issue mapping counts, and skipped issue files.
- [x] Validate document kinds and stability values.

### Task 3: CLI Import Command

- [x] Make `multica spec sync --from-files` scan local `.spec`.
- [x] POST snapshot to the backend with normal CLI auth/workspace headers.
- [x] Print summary as table or json.

### Task 4: Backend Export Endpoint

- [x] Add `GET /api/spec/sync/to-files`.
- [x] Reconstruct epics/modules/documents/decisions from backend Spec Memory.
- [x] Reconstruct issue execution state and primary/related mappings from backend issue state.
- [x] Return a file-shaped snapshot compatible with the local `.spec` writer.

### Task 5: CLI Export Command

- [x] Make `multica spec sync --to-files` fetch backend Spec Memory.
- [x] Write the returned snapshot to local `.spec`.
- [x] Skip existing local files by default and report skipped counts.
- [x] Add `--force` for explicit local overwrite.

### Task 6: Verification

- [x] Unit test snapshot parsing.
- [x] Unit test snapshot writing and overwrite protection.
- [x] Handler test import endpoint.
- [x] Handler test export endpoint.
- [x] CLI test explicit command behavior.
- [x] Run focused Go tests.
