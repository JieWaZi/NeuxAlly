---
name: ally
description: Use NeuxAlly-provided capabilities through allycli for message and file delivery, workflow authoring or execution, team coordination, memory, plugins, and structured user questions. Apply when a task needs a platform operation advertised by allycli; ordinary file editing or shell work alone does not require this skill.
---

# NeuxAlly

Use `allycli` to perform the platform operations needed for the user's task.
The application supplies the binary and execution context. Do not install a
replacement, invent credentials, or call desktop administration endpoints.
If command lookup is ambiguous, invoke the quoted absolute executable supplied
in `ALLYCLI_BINARY_PATH` using the current shell's environment syntax.

## Discover only what is needed

- At first use in an execution, run `allycli capabilities`. Its result is the
  current authorized surface, not permission to perform extra work.
- For workflow creation/editing, run `allycli workflow authoring prepare
  --primitives <actual DSL calls>` when advertised. It returns the minimal
  guides/SDK, current source and save contract together. Reuse that result.
- For a contract still missing, run `allycli describe <operation-id>`.
  Use its schema, limits, completion meaning and advertised legal values.
- Read a relevant `allycli guide <guide-id>` only when its content was not
  already included in preparation. Live contracts determine allowed syntax.
- Reuse confirmed decisions. Investigate discoverable facts before asking.
  Do not run all discovery commands or read every guide by default.

## Call and verify

- Use simple flags for short scalar values and `--input <file>` or
  `--input -` for structured input. Put source text and secrets in files or
  stdin rather than shell command strings. Default output is JSON.
- For mutations, choose a stable `--request-id` before the first call.
  Preserve that ID and the same payload when resolving an uncertain result.
- Inspect `completion` and the resource's business state. An accepted run,
  registered delivery, or submitted result candidate is not final success.
  Follow the receipt's `nextStep` when supplied.
- Use the resource's `get` or bounded `wait` command when final confirmation
  is needed. A wait timeout changes neither the operation nor the resource.
- If a mutation loses its response, query `allycli receipts get` with the
  same operation and request ID. Do not create a new request just to retry.
- On a denied or expired context, stop the dependent action. Do not switch
  identities, modify the context file, or use a broader endpoint.
  `refresh_context` requires the host to provide a new authorized execution
  context; it is not an agent-callable renewal command.

## Preserve user intent

Sending externally, starting work, retrying failed work, and changing shared
resources must stay within the user's request and the live grant. Do not ask
for settled decisions again or treat silence, skipping, or cancellation as
approval. Use structured questions when a consequential choice is missing.

Use real file references for requested artifacts. Submit the actual candidate
or report through the advertised result operation; a final chat message is
not a submission receipt. Tell the user what was actually saved, accepted,
completed, or delivered, preserving failures and uncertainty.

## System plugins

When advertised, use `plugins.catalog.search` and `plugins.catalog.get` to inspect the fixed software catalog, then `plugins.installations.install` for the current partner. Preserve the request ID on retries. Complete any returned authorization session in the plugin interface; temporary authorization material is not a chat result. An installed receipt records the system relationship and enables the next execution. Never replace this operation with native CLI global plugin installation or edit the user's Codex or Claude global registry.
