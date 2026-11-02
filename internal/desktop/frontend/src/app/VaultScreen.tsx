import type { FormEvent } from "react";
import type { VaultStatus } from "../types/app";
import appIcon from "../assets/images/hoplink-icon.png";

type Props = {
  vaultStatus: VaultStatus | null;
  vaultPassword: string;
  vaultMessage: string;
  onPasswordChange: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
};

export function VaultScreen({
  vaultStatus,
  vaultPassword,
  vaultMessage,
  onPasswordChange,
  onSubmit,
}: Props) {
  return (
    <div className="vault-screen">
      <form className="vault-card" onSubmit={onSubmit}>
        <img src={appIcon} alt="HopLink" />
        <div className="vault-eyebrow">ЛОКАЛЬНОЕ ХРАНИЛИЩЕ</div>
        <h1>
          {vaultStatus?.needsSetup
            ? "Задайте пароль"
            : "Разблокировать HopLink"}
        </h1>
        <p>
          {vaultStatus?.needsSetup
            ? vaultStatus.hasData
              ? "Старые настройки будут зашифрованы этим паролем. Он не сохраняется в файл."
              : "Создайте пароль для шифрования настроек. Он будет храниться только в памяти приложения."
            : "Введите пароль, которым зашифрован файл настроек."}
        </p>
        {vaultStatus?.error && (
          <div className="vault-error">{vaultStatus.error}</div>
        )}
        <label className="field">
          Пароль
          <input
            autoFocus
            type="password"
            value={vaultPassword}
            onChange={(event) => onPasswordChange(event.target.value)}
            autoComplete="current-password"
            required
            minLength={1}
          />
        </label>
        {vaultMessage && (
          <div className="vault-error">
            {vaultMessage.includes("неверный пароль")
              ? "Неверный пароль. Проверьте его и попробуйте ещё раз."
              : vaultMessage}
          </div>
        )}
        <button
          className="btn primary vault-submit"
          type="submit"
          disabled={!vaultPassword}
        >
          {vaultStatus?.needsSetup
            ? "Создать зашифрованное хранилище"
            : "Войти"}
        </button>
        <small>
          Пароль не записывается на диск. Без него содержимое настроек
          восстановить нельзя.
        </small>
      </form>
    </div>
  );
}

export function needsVaultScreen(vaultStatus: VaultStatus | null) {
  return (
    !vaultStatus ||
    vaultStatus.locked ||
    (vaultStatus.needsSetup && vaultStatus.hasData)
  );
}
