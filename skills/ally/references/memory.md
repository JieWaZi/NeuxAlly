# Memory

Use memory when prior durable facts help the current task, or the user explicitly asks to remember, correct or forget information. Search current visible memory before saving a duplicate. Choose partner scope for reusable cross-project facts and project scope only when the current context actually binds that project.

Write concise stable knowledge. Do not save access tokens, transient progress, whole conversation transcripts, or unverified guesses as durable facts. An explicit user request to remember or forget requires a real operation and receipt, not a verbal promise.

Use `memory entries search/get/create/update/forget`. Search is bounded retrieval and returns its actual semantic or keyword mode; do not call topK results complete coverage. Update replaces a known entry's content; forget removes current knowledge while retaining the change event. IDs must come from visible results, and cannot change partner/project ownership.
