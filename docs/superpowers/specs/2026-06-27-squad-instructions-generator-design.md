# Squad Instructions Generator Design

## Goal

Add a product feature that helps users generate useful `squad.instructions` from the squad's current agents, roles, descriptions, and skills.

The first implementation is deterministic template generation. The API shape reserves room for a later AI polish mode, but this version does not call a model. This keeps the feature fast, offline-friendly, testable, and aligned with the existing squad leader briefing pipeline.

## Existing Context

Squad leader tasks already receive a briefing that includes:

- `## Squad Operating Protocol`
- `## Squad Roster`
- optional `## Squad Instructions (<squad name>)`

The custom instructions are stored on `squad.instructions` and appended at task claim time. The UI already has a Squad detail Instructions tab with an editor and save button.

The generated draft should update only the editable instructions content. It should not modify the hard-coded Operating Protocol or generated roster.

## User Experience

On the Squad detail Instructions tab, add a secondary action:

`Generate draft`

When clicked:

1. The client calls a new generate endpoint for the current squad.
2. The endpoint returns generated markdown plus metadata such as generation mode.
3. The editor is populated with the returned draft.
4. The user can edit the draft.
5. Existing `Save` persists it through `api.updateSquad`.

The feature never silently overwrites persisted instructions. If the editor has unsaved edits, the UI asks for confirmation before replacing the local draft.

## Generated Content

The deterministic generator emits concise markdown focused on delegation behavior, for example:

```md
## Delegation Strategy

- Route frontend UI, interaction, and shared view work to Frontend Agent.
- Route backend API, scheduler, database, and integration work to Backend Agent.
- If a task spans multiple areas, ask the first owner to split follow-up child issues instead of doing unrelated work.
- Prefer creating one child issue per assignee when work can proceed independently.
- Do not delegate to archived agents or members outside this squad.

## Member Routing

- Frontend Agent: Use for UI work. Skills: react, frontend, shadcn.
- Backend Agent: Use for Go/API/database work. Skills: go, postgres, sqlc.
```

The exact text is assembled from:

- squad name and description
- squad member role
- agent name and description
- agent skills
- whether the member is an agent or human

Archived agents are excluded. Human members may be listed as escalation/review contacts, but not as runnable agent assignees.

## API

Add:

```http
POST /api/squads/{id}/instructions/generate
```

Request:

```json
{
  "mode": "template"
}
```

Response:

```json
{
  "instructions": "## Delegation Strategy\n...",
  "mode": "template",
  "warnings": []
}
```

`mode` accepts only `template` in the first version. A later `polish` or `ai` mode can reuse the same endpoint.

Permission should match updating squad instructions. The handler must verify workspace membership and squad access through existing squad loaders.

## Backend Design

Add a small generator in the squad handler/service boundary:

- Load the squad by workspace.
- Load members via `ListSquadMembers`.
- Load agent rows and skill names for agent members.
- Build a stable intermediate view model.
- Render markdown using deterministic rules.

The generator should be pure enough to test without HTTP setup where practical. HTTP tests should cover:

- generated instructions include agent roles and skills
- archived agents are excluded
- empty squad produces a useful fallback
- invalid mode returns 400
- unauthorized or missing squad follows existing handler behavior

## Frontend Design

Add an API client method:

```ts
generateSquadInstructions(id, { mode: "template" })
```

Add zod schema parsing for the response.

In `SquadInstructionsTab`:

- Add a `Generate draft` button beside `Save`.
- Show loading state on the button.
- If the editor is dirty, confirm replacement before applying the generated draft.
- On success, replace the editor content and mark it dirty so the user must explicitly save.
- On failure, show a toast and leave current content unchanged.

The shared view remains platform-neutral and uses existing `api` and TanStack Query patterns.

## Future AI Polish

The endpoint shape intentionally leaves room for:

```json
{ "mode": "polish" }
```

That later mode should not introduce a direct one-off model client if the product can reuse the existing runtime/agent task architecture. The template mode remains the fallback when no model is available.

## Testing

Backend:

- Go tests for generator behavior.
- Handler tests for request validation and permissions.

Frontend:

- `packages/core` schema/client test for the new response shape.
- `packages/views` component test that clicking `Generate draft` fills the editor and requires Save.
- Dirty replacement test to avoid losing unsaved instructions silently.

## Out of Scope

- Automatically writing instructions for existing squads without user review.
- Calling OpenAI/Claude directly from the server.
- Changing the hard-coded Squad Operating Protocol.
- Creating child issues automatically from generated instructions.
