import type { ServerDraft } from "../modals/ServerModal";

export function emptyServerDraft(): ServerDraft {
  return {
    name: "",
    host: "",
    sshPort: 22,
    sshUser: "root",
    sshPassword: "",
    sshPrivateKey: "",
    sshKeyPassphrase: "",
  };
}
