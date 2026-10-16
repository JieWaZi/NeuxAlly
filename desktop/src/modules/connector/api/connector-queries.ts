import type { QueryClient } from "@tanstack/react-query";

export {
  createWakerConnectorMutation,
  deleteWakerConnectorMutation,
  getWakerConnectorOptions,
  getWakerConnectorQueryKey,
  listWakerConnectorsOptions,
  listWakerConnectorsQueryKey,
  setWakerConnectorEnabledMutation,
  testWakerConnectorMutation,
  updateWakerConnectorMutation,
} from "@/shared/api/generated/@tanstack/react-query.gen";

const visibleConnectorQueryIDs = new Set([
  "listWakerConnectors",
  "getWakerConnector",
]);

export async function invalidateVisibleConnectorQueries(
  queryClient: QueryClient,
): Promise<void> {
  await queryClient.invalidateQueries({
    predicate: (query) => {
      const descriptor = query.queryKey[0] as { _id?: string } | undefined;
      return Boolean(
        descriptor?._id && visibleConnectorQueryIDs.has(descriptor._id),
      );
    },
  });
}
