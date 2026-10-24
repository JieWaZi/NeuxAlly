import { deleteApiKey } from "./generated/sdk.gen";
import {
  getApiServiceOptions,
  getApiServiceQueryKey,
  updateApiServiceMutation,
} from "./generated/@tanstack/react-query.gen";
import {
  withLocalErrorSurface,
  withLocalErrorMutation,
} from "./local-error-surface";

/** 设置页与自动任务共同读取同一个服务状态，不能派生另一份监听配置。 */
export const integrationServiceOptions = () =>
  getApiServiceOptions(withLocalErrorSurface({}));
export const integrationServiceQueryKey = getApiServiceQueryKey;
export const integrationServiceMutation = () =>
  withLocalErrorMutation(updateApiServiceMutation());

/** 删除固定版本的密钥；错误只显示在当前删除对话框。 */
export const deleteIntegrationKey = (keyId: string, expectedRevision: number) =>
  deleteApiKey<true>(
    withLocalErrorSurface({
      path: { keyId },
      body: { expectedRevision },
      throwOnError: true,
    }),
  );
