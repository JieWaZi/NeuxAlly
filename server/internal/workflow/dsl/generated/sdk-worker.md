# Workflow SDK: worker

Worker chats use the same managed runtime workspace and native instruction-file loading as ordinary chats. Their runtime working directories are isolated; the approved business directory is shared through project links and additional directory authorization. Build shared file paths from flowContext.scope.workingDirectory and pass those absolute paths to worker prompts and results. Never assume two workers have the same process working directory. Use run-specific output paths, require actual tools to create and read deliverable files, and verify existence and nonempty content.

A structured worker must call allycli workflow results submit with a stable --request-id and an input JSON object whose data matches its schema. A submission is only a candidate until its Execution succeeds. Do not rely on JSON pasted into a final reply. For a root file schema, data is the actual file path. Add routing or summary fields only for real downstream consumers.

Workers have no independent timeout; worker timeoutMs is unsupported. User cancellation and actual execution failure remain effective.

Every worker, including workers inside loops, branches and parallel groups, requires exactly one output declaration: meta.nodeResultPages[id] for composed in-app reading, meta.nodeResultFiles[id] with a readable title for actual file delivery, or meta.nodeResultPageOmissions[id] with a concrete reason for internal technical/control data without reading value or output the user explicitly declined to display. Intermediate output is not automatically exempt. File delivery uses static type:string, format:file schemas, or arrays/objects of these references, and the existing file viewer. Keep the report's prose, evidence and coverage in the actual file. Select the presentation feature only for composed pages; preparation includes its Catalog. Reuse that content and query the presentation topic only if it is missing. Revisit the declaration when the user changes the deliverable.

```typescript
type Json = null | boolean | number | string | Json[] | { [key: string]: Json };
type Schema = { [key: string]: Json };
interface VisualNode { id: string; label: string; phase: string; inputs?: string[]; outputs?: string[] }
interface WakerResolve { kind: 'waker'; wakerId: string; runtimeId?: string; accountId?: string; model?: string; reasoning?: string; permissionLevel?: 'default'|'auto'|'full-access' }
interface ToolPolicy { connectors?: {mode:'inherit'} | {mode:'allowlist';connectorIds:string[]}; nativeTools?: {mode:'inherit'} | {mode:'allowlist';capabilityIds:string[]} }
interface WorkerOptions { resolve: WakerResolve; schema?: Schema; toolPolicy?: ToolPolicy; skippable?: boolean; retry?: {maxAttempts:number;safety:'pure'|'idempotent';idempotencyKey?:string} }
declare const flowContext: Readonly<{workflowId:string;revisionId:string;runId:string;workflowName:string;parentRunId:string|null;retrySourceRunId:string|null;triggerSource:'manual'|'schedule'|'local_api'|'agent'|'child';scope:{projectId:string|null;workingDirectory:string}}>;
declare function worker<T extends Json = string>(prompt: string, options: VisualNode & WorkerOptions): Promise<T | null>;
```
