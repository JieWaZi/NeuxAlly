# Workflow Execution

Use `workflow definitions list` and `workflow definitions describe` to read the actual authorized definition, revision and input/output contracts. A known ID is not permission to invoke it. Do not invent resources or widen the execution configuration.

Prepare only inputs required by the selected immutable revision. A file reference must point to an actual permitted file; a path is not file content. Use the server's file contract rather than guessing types from extensions. Supply a stable request ID to `workflow runs start` and keep its receipt.

Relative file inputs resolve from the CLI process's current directory. Only schema-declared files are resolved and frozen; ordinary strings stay unchanged. The receipt binds the frozen file bytes: changing a file under the same request ID is a conflict. If the original file is no longer available or the start outcome is uncertain, query `allycli receipts get --operation workflow.runs.start --request-id <original-id>`.

Acceptance is not completion. Read the real run with `workflow runs get` or use bounded `workflow runs wait`. Preserve failure, partial evidence and uncertainty. A wait timeout does not cancel the run. Cancel only the authorized run when requested; cancellation cannot undo completed external effects.

Read output files through actual references in the result contract. Do not treat an arbitrary JSON string or final chat message as a successful external result.

`get.run.cancellationReason` and `wait.resource.run.cancellationReason` describe a requested stop: `actor`, `source` and `requestedAt` preserve the first request. `completed` only means a terminal state; read the run status and reason for the outcome.

For a successful run with declared final files, use the discovered `workflow runs deliver` operation with the original run ID and a stable delivery request ID. `files` describes declared outputs and availability; `attachments` in the delivery receipt contains the registered chat file references (`name`, `path`, `mediaType`, `managed`). Reading `get` or `wait` does not register attachments. If delivery or message registration is uncertain, query `allycli receipts get --operation workflow.runs.deliver --request-id <original-delivery-id>` to recover the same receipt and original message registration. Delivery failure does not change the run's terminal result.
