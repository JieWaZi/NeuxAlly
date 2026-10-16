# Workflow SDK: parallel

Await workers, human inputs, actions and child results. Parallel returns in input order. Pipeline advances each item immediately after its own preceding stage, then joins all items in original order. A null stage result is ordinary JSON, not a failure. Explicit throw stops the failed item.

```typescript
type Json = null | boolean | number | string | Json[] | { [key: string]: Json };
type Schema = { [key: string]: Json };
interface VisualNode { id: string; label: string; phase: string; inputs?: string[]; outputs?: string[] }
interface GroupOptions { id:string; failureMode?:'collect-null'|'fail-fast'; concurrency?:number }
declare function parallel<T extends Json>(tasks: Array<() => Promise<T>>, options: GroupOptions): Promise<Array<T | null>>;
```
