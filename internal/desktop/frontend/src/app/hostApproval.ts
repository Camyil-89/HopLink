export type HostApprovalRequest = { host: string; fp: string };

export function parseHostApproval(message: string): HostApprovalRequest | null {
  const fingerprint = message.match(/SHA256:[A-Za-z0-9+/]{43}/)?.[0];
  const host = message.match(/(?:для|for)\s+([^\s()]+)/i)?.[1];
  return fingerprint && host ? { host, fp: fingerprint } : null;
}

export function isSynchronizationCancelled(message: string) {
  return /synchronization cancelled|context canceled|context cancelled|синхронизация отменена/i.test(
    message,
  );
}
