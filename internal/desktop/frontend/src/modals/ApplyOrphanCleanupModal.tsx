import type { Server } from "../types/app";

export type OrphanCleanupChoice = "remove" | "exclude";

type Props = {
  servers: Server[];
  choices: Record<string, OrphanCleanupChoice>;
  onChoice: (serverId: string, choice: OrphanCleanupChoice) => void;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ApplyOrphanCleanupModal({
  servers,
  choices,
  onChoice,
  onCancel,
  onConfirm,
}: Props) {
  return (
    <div className="modal-backdrop">
      <div
        className="modal apply-orphan-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="apply-orphan-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <h2 id="apply-orphan-title">Узлы сняты с карты</h2>
            <p>
              На этих серверах ещё числится HopLink, но их нет в текущей
              цепочке. Выберите для каждого, удалять ли сервис по SSH при
              синхронизации.
            </p>
          </div>
        </div>
        <ul className="apply-orphan-list">
          {servers.map((server) => (
            <li key={server.id}>
              <div className="apply-orphan-server">
                <b>{server.name || server.host}</b>
                <small>
                  {server.sshUser}@{server.host}:{server.sshPort}
                </small>
              </div>
              <div
                className="apply-orphan-choices"
                role="radiogroup"
                aria-label={`Очистка ${server.name || server.host}`}
              >
                <label className="apply-orphan-option">
                  <input
                    type="radio"
                    name={`orphan-${server.id}`}
                    checked={choices[server.id] === "remove"}
                    onChange={() => onChoice(server.id, "remove")}
                  />
                  <span>Удалить сервис по SSH</span>
                </label>
                <label className="apply-orphan-option">
                  <input
                    type="radio"
                    name={`orphan-${server.id}`}
                    checked={choices[server.id] === "exclude"}
                    onChange={() => onChoice(server.id, "exclude")}
                  />
                  <span>Не удалять (исключить)</span>
                </label>
              </div>
            </li>
          ))}
        </ul>
        <p className="field-hint apply-orphan-hint">
          «Не удалять» убирает сервер из списка установки без подключения.
          Сервис на сервере останется, пока вы не удалите его вручную.
        </p>
        <div className="modal-actions">
          <button type="button" className="btn secondary" onClick={onCancel}>
            Отмена
          </button>
          <button type="button" className="btn" onClick={onConfirm}>
            Продолжить синхронизацию
          </button>
        </div>
      </div>
    </div>
  );
}