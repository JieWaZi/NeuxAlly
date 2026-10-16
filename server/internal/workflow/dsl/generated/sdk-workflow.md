# Workflow SDK: workflow

Await workers, human inputs, actions and child results. Parallel returns in input order. Pipeline advances each item immediately after its own preceding stage, then joins all items in original order. A null stage result is ordinary JSON, not a failure. Explicit throw stops the failed item.

```typescript
type Json = null | boolean | number | string | Json[] | { [key: string]: Json };
type Schema = { [key: string]: Json };
interface VisualNode { id: string; label: string; phase: string; inputs?: string[]; outputs?: string[] }
interface ChildOptions { args?:Json; revisionId?:string; timeoutMs?:number }
interface ChildResult { runId:string; revisionId:string; status:'succeeded'; result:Json }
/** Opaque durable handle passed only between startWorkflow and joinWorkflow. */
declare const workflowChildHandleBrand: unique symbol;
interface ChildHandle { readonly [workflowChildHandleBrand]: true }
declare function workflow(workflowId: string, options: VisualNode & ChildOptions): Promise<ChildResult>;
declare function startWorkflow(workflowId: string, options: VisualNode & ChildOptions & { mode: 'joined' | 'detached' }): ChildHandle;
declare function joinWorkflow(handle: ChildHandle): Promise<ChildResult>;
```
