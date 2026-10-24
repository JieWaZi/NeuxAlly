import type { GithubComJieWaZiNeuxAllyServerConstantsImPlatform } from "@/shared/api/generated/types.gen";

export type {
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsImConnection as IMConnection,
  GithubComJieWaZiNeuxAllyServerRepositoriesModelsImBinding as IMBinding,
  GithubComJieWaZiNeuxAllyServerServicesImLoginProgress as IMLogin,
  GithubComJieWaZiNeuxAllyServerServicesImConnectionConfiguration as ConnectionConfiguration,
} from "@/shared/api/generated/types.gen";

export type IMPlatform = Exclude<
  GithubComJieWaZiNeuxAllyServerConstantsImPlatform,
  ""
>;
