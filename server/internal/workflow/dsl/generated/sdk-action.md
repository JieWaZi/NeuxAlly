# Workflow SDK: action

Await workers, human inputs, actions and child results. Parallel returns in input order. Pipeline advances each item immediately after its own preceding stage, then joins all items in original order. A null stage result is ordinary JSON, not a failure. Explicit throw stops the failed item.

HTTP actions default to 30000 ms. Command/script actions default to 600000 ms, with the same hard maximum. Omit timeoutMs unless the user agreed to a shorter deadline.

```typescript
type Json = null | boolean | number | string | Json[] | { [key: string]: Json };
type Schema = { [key: string]: Json };
interface VisualNode { id: string; label: string; phase: string; inputs?: string[]; outputs?: string[] }
interface ActionOptions { input?:Json; timeoutMs?:number; url?:string; method?:string; headers?:Record<string,string>; body?:Json; secretHeaders?:Record<string,{secretRef:string}>; args?:string[]; env?:Record<string,string>; secretEnv?:Record<string,{secretRef:string}>; schema?:Schema }
interface ActionResult { data:Json; durationMs:number; http?:{status:number;headers:Record<string,string>;body:string} }
type ActionDeclaration = {id:string;label:string;description:string;risk:'read'|'write'|'external';inputSchema?:Schema;outputSchema?:Schema} & ({type:'http';allowedOrigins:string[];methods:string[]} | {type:'script';executable:string;workingDirectory:'workflow_scope'} | {type:'builtin';handler:string;connectorIds?:string[];imTargetIds?:string[]});
declare function action(actionId: string, options: VisualNode & ActionOptions): Promise<ActionResult>;
```
