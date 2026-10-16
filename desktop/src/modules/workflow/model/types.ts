export type {
  GithubComJieWaZiNeuxAllyServerRepositoriesControllersWorkflowNodeCount as WorkflowNodeCount,
  GithubComJieWaZiNeuxAllyServerInternalWorkflowDslManifest as Manifest,
  GithubComJieWaZiNeuxAllyServerInternalWorkflowDslNode as WorkflowNode,
  GithubComJieWaZiNeuxAllyServerInternalWorkflowDslPhase as WorkflowPhase,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsWorkflowRun as WorkflowRun,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsWorkflowNodeInstance as WorkflowInstance,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsWorkflowHumanRequest as HumanRequest,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsWorkflowDefinition as WorkflowDefinition,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsWorkflowRevision as WorkflowRevision,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsWorkflowExecutionDefaults as ExecutionDefaults,
} from "@/shared/api/generated/types.gen";

export function record(value: unknown): Record<string, unknown> | undefined {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? Object.fromEntries(Object.entries(value))
    : undefined;
}
export function jsonText(value: unknown): string {
  return JSON.stringify(value, null, 2) ?? "null";
}
