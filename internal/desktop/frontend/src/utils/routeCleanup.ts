import type { Server } from "../types/app";

export function pendingRouteCleanup(
  mapServers: Server[],
  deployed: Server[] | undefined,
): Server[] {
  if (!deployed?.length) return [];
  const onMap = new Set(mapServers.map((s) => s.id));
  const onMapHost = new Set(
    mapServers.map((s) => `${s.host.toLowerCase()}:${s.sshPort}`),
  );
  return deployed.filter((s) => {
    if (onMap.has(s.id)) return false;
    const key = `${s.host.toLowerCase()}:${s.sshPort}`;
    return !onMapHost.has(key);
  });
}