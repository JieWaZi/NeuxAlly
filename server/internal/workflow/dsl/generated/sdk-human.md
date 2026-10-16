# Workflow SDK: human

Await workers, human inputs, actions and child results. Parallel returns in input order. Pipeline advances each item immediately after its own preceding stage, then joins all items in original order. A null stage result is ordinary JSON, not a failure. Explicit throw stops the failed item.

Human defaults do not submit or approve. Approvals require a positive explicit value; skip and timeout preserve their source. Actions use separate operation permissions.

```typescript
type Json = null | boolean | number | string | Json[] | { [key: string]: Json };
type Schema = { [key: string]: Json };
interface VisualNode { id: string; label: string; phase: string; inputs?: string[]; outputs?: string[] }
interface HumanOptions { options?: Array<{value:string;label:string}>; multiSelect?:boolean; schema?:Schema; defaultValue?:Json; timeoutMs?:number; allowSkip?:boolean }
interface HumanAnswer { value:Json; skipped:boolean; source:'user'|'skip'|'timeout'; requestId:string }
declare function askUser(prompt: string, options: VisualNode & HumanOptions): Promise<HumanAnswer>;
```
