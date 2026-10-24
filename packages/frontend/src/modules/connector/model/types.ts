import type {
  GithubComJieWaZiNeuxAllyServerApisMcpConnectorResource,
  GithubComJieWaZiNeuxAllyServerApisMcpConnectorResponse,
  GithubComJieWaZiNeuxAllyServerApisMcpRuntimeCompatibility,
  GithubComJieWaZiNeuxAllyServerApisMcpSecretInput,
} from "@/shared/api/generated/types.gen";

export type ConnectorTransport = "stdio" | "streamable_http" | "sse";
export type ConnectorScope = "built_in" | "public" | "private";
export type ConnectorTestStatus = "untested" | "succeeded" | "failed";
export type ConnectorActivationPolicy =
  "per_execution" | "team_context" | "workflow_context";

export type ConnectorView =
  GithubComJieWaZiNeuxAllyServerApisMcpConnectorResponse;
export type ConnectorRecord =
  GithubComJieWaZiNeuxAllyServerApisMcpConnectorResource;
export type ConnectorCompatibility =
  GithubComJieWaZiNeuxAllyServerApisMcpRuntimeCompatibility;

export type SecretField = GithubComJieWaZiNeuxAllyServerApisMcpSecretInput & {
  configured?: boolean;
};

export type ConnectorFormValue = {
  argumentsText: string;
  command: string;
  description: string;
  environment: SecretField[];
  headers: SecretField[];
  name: string;
  timeoutMS: string;
  transport: ConnectorTransport;
  url: string;
};

export type ConnectorSummary = {
  activationPolicy: ConnectorActivationPolicy;
  affectedWakerCount: number;
  compatibility: ConnectorCompatibility[];
  description: string;
  enabled: boolean;
  id: string;
  name: string;
  scope: ConnectorScope;
  testStatus: ConnectorTestStatus;
  toolCount: number;
  transport: ConnectorTransport;
};
