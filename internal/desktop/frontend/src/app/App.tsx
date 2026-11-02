import { AppMain } from "./AppMain";
import { AppModals } from "./AppModals";
import { needsVaultScreen, VaultScreen } from "./VaultScreen";
import { useHoplinkApp } from "./useHoplinkApp";

export default function App() {
  const app = useHoplinkApp();

  if (needsVaultScreen(app.vaultStatus)) {
    return (
      <VaultScreen
        vaultStatus={app.vaultStatus}
        vaultPassword={app.vaultPassword}
        vaultMessage={app.vaultMessage}
        onPasswordChange={app.setVaultPassword}
        onSubmit={app.submitVault}
      />
    );
  }

  return (
    <>
      <AppMain app={app} />
      <AppModals app={app} />
    </>
  );
}
