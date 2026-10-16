# Structured User Questions

Ask only for a missing consequential decision that cannot be established from the task or available evidence. Reuse explicit instructions and already settled choices. Ordinary implementation choices can be made within the user's delegated scope.

Use the live `interaction.questions.create` contract for supported fields, choices and limits; describe it if missing. Give fields ordinary, distinct property names and set `allowSkip` according to the decision. The question belongs to the current execution and the user answers through the application. Read its answer state with get or bounded wait.

Do not submit answers yourself, infer consent from silence, or treat skipped, cancelled, invalidated or still pending as approval. A wait timeout stops observation only. Continue independent work where possible; keep dependent work waiting for the actual answer. A business question does not replace tool authorization or user acceptance of a completed task.
