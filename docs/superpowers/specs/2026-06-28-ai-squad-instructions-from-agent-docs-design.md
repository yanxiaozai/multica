# AI Squad Instructions From Agent Docs Design

## Goal

Generate `squad.instructions` by having AI read and summarize the squad members' agent documents, then produce leader-facing delegation rules.

This is different from the existing deterministic template generator. The template generator assembles known fields into markdown. This feature asks an agent/runtime to interpret each member's `instructions` document, identify strengths and boundaries, and write a concise operating guide for the squad leader.

## Existing Context

The first squad instructions generator already adds:

- a synchronous `POST /api/squads/{id}/instructions/generate` endpoint for `mode: "template"`
- a shared UI button that fills the Instructions editor with an editable draft
- deterministic backend rendering from squad members, agent descriptions, and skills

Agent documents are available in the database through `agent.instructions`. Claude agent import stores the original file body in that field, so the AI summarizer can use the stored agent record rather than reading local filesystem paths at generation time.

The product's AI execution model is runtime/daemon based. Server handlers enqueue work for an agent runtime; they do not directly call OpenAI or Claude APIs.

## User Experience

On the Squad detail Instructions tab, keep the current template action and add an AI action:

- `Generate template`
- `AI summarize agents`

When the user clicks `AI summarize agents`:

1. The UI sends the current squad id and optional current editor content.
2. The backend creates a generation job and enqueues it for a selected agent.
3. The UI shows an in-progress state and polls or subscribes for completion.
4. When the job completes, the generated instructions replace the editor draft after confirmation if the editor has unsaved changes.
5. The user reviews and explicitly clicks `Save`.

The feature must not silently persist generated instructions. Generated text is always a draft until the user saves it.

## AI Input

The AI prompt should include only the data needed to summarize routing behavior:

- squad name and description
- existing squad instructions, if any
- current editor draft, if provided
- leader agent name, description, skills, and instructions
- each runnable squad member agent:
  - name
  - member role
  - description
  - skill names
  - instructions document
- human squad members as escalation contacts only

Archived agents are excluded. Secrets and runtime environment values are not included. `custom_env` remains out of scope.

To control prompt size, each agent document should be bounded before enqueue:

- include the full document when it is short
- for long documents, include a clearly marked truncated excerpt
- include name, description, role, and skills even when instructions are empty

## AI Output Contract

The AI must return markdown that is suitable to store directly in `squad.instructions`.

Required sections:

```md
## Delegation Strategy

## Member Routing

## Coordination Rules

## Escalation
```

The output should:

- summarize what each agent is best suited for
- name tasks the agent should not receive when the document makes that clear
- describe how the leader should split cross-domain work
- avoid mentioning implementation internals of Multica
- avoid inventing skills that are not supported by the agent documents
- avoid changing the fixed Squad Operating Protocol, which is injected separately at task claim time

The AI should not create issues, assign work, or modify agents. Its only artifact is generated instructions text.

## Agent Selection

Use the squad leader agent by default because the generated instructions are for the leader's future routing behavior.

If the leader has no runtime or is archived, the request should fail with a clear user-facing error. A later version can allow selecting a separate summarizer agent, but the first AI version should keep one predictable path.

## Backend Design

Add an asynchronous squad instructions generation path rather than making the existing template endpoint block on AI output.

Recommended API shape:

```http
POST /api/squads/{id}/instructions/generation-jobs
```

Request:

```json
{
  "mode": "agent_docs",
  "draft": "optional current editor text"
}
```

Response:

```json
{
  "job_id": "uuid",
  "task_id": "uuid",
  "status": "queued"
}
```

Add:

```http
GET /api/squads/{id}/instructions/generation-jobs/{job_id}
```

Response:

```json
{
  "id": "uuid",
  "status": "queued|running|completed|failed|cancelled",
  "instructions": "## Delegation Strategy\n...",
  "error": null
}
```

The backend needs a durable job row so the UI can poll a clean result without scraping chat messages or creating visible chat sessions. The job links to the underlying `agent_task_queue` task.

## Runtime Integration

Create a new task context type, for example:

```json
{
  "type": "squad_instructions_generation",
  "workspace_id": "uuid",
  "squad_id": "uuid",
  "generation_job_id": "uuid"
}
```

The daemon already passes task context into runtime preparation. Extend the task briefing logic so this context produces a prompt that asks the selected agent to generate only the markdown instructions.

On task completion, detect `context.type == "squad_instructions_generation"` and copy the final output into the generation job result. Do not create an issue comment or chat message for this task.

## Frontend Design

Add core API methods and schemas:

- `createSquadInstructionsGenerationJob`
- `getSquadInstructionsGenerationJob`

In `SquadInstructionsTab`:

- keep the existing template generation action
- add an AI summarize action
- disable the AI action while a job is active
- show `Queued`, `Running`, `Completed`, or failure text in the tab action area
- poll the job until terminal state
- on completion, replace the editor content and mark it dirty

The shared view stays platform-neutral and uses existing TanStack Query patterns.

## Error Handling

User-facing failures:

- leader agent is archived
- leader agent has no runtime
- runtime is offline or cannot claim the task
- task fails
- generated output is empty
- generated output is too large for `squad.instructions`

When AI generation fails, keep the current editor content unchanged and show a toast with the error. The template generator remains available as a fallback.

## Testing

Backend:

- job creation rejects non-admin/non-owner callers
- job creation rejects squads with no usable leader runtime
- task context includes bounded agent documents and excludes archived agents
- completion copies task output into the job result
- failed task updates job status and error

Frontend:

- clicking AI summarize creates a job and shows progress
- completed job fills the editor but does not save automatically
- dirty editor replacement asks for confirmation
- failed job leaves current content unchanged

## Out of Scope

- Direct OpenAI or Claude API calls from the server
- Reading local Claude agent files during generation
- Automatically saving generated instructions
- Selecting a separate summarizer agent
- Streaming partial output into the editor
- Summarizing agents outside the current squad
