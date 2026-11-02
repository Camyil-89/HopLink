import { useMemo, useState } from "react";
import { Copy, QrCode, X } from "lucide-react";
import { qrMatrix } from "../utils/qr";

type Props = {
  name: string;
  links: { dns: string; ip: string };
  onClose: () => void;
};

function QRView({ value }: { value: string }) {
  const matrix = useMemo(() => qrMatrix(value), [value]);
  const quiet = 4,
    size = matrix.length + quiet * 2;
  return (
    <svg
      className="user-qr"
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label="QR-код ссылки подключения"
      shapeRendering="crispEdges"
    >
      <rect width={size} height={size} fill="white" />
      {matrix.flatMap((row, r) =>
        row.map((on, c) =>
          on ? (
            <rect
              key={`${r}-${c}`}
              x={c + quiet}
              y={r + quiet}
              width="1"
              height="1"
              fill="#171b25"
            />
          ) : null,
        ),
      )}
    </svg>
  );
}

function LinkCard({ title, value }: { title: string; value: string }) {
  const [showQR, setShowQR] = useState(false);
  const [copied, setCopied] = useState(false);
  return (
    <section className="user-link-option">
      <div className="user-link-option-head">
        <div>
          <b>{title}</b>
          <small>{value ? "Ссылка подключения Hiddify" : "DNS не задан"}</small>
        </div>
        {value && (
          <div className="user-link-option-actions">
            <button
              className="btn secondary"
              onClick={() => {
                void navigator.clipboard.writeText(value).then(() => {
                  setCopied(true);
                  window.setTimeout(() => setCopied(false), 1600);
                });
              }}
            >
              <Copy size={14} />
              {copied ? "Скопировано" : "Копировать"}
            </button>
            <button
              className="btn secondary"
              onClick={() => setShowQR((v) => !v)}
            >
              <QrCode size={14} />
              {showQR ? "Скрыть QR" : "Показать QR"}
            </button>
          </div>
        )}
      </div>
      {value && (
        <>
          <code className="user-link-value">{value}</code>
          {showQR && (
            <div className="user-qr-wrap">
              <QRView value={value} />
            </div>
          )}
        </>
      )}
    </section>
  );
}

export function UserLinksModal({ name, links, onClose }: Props) {
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <section
        className="modal user-links-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="user-links-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <div>
            <h2 id="user-links-title">Ссылки пользователя</h2>
            <p>{name} · адреса заменяются автоматически из карты маршрута</p>
          </div>
          <button className="icon-btn" onClick={onClose} title="Закрыть">
            <X size={17} />
          </button>
        </div>
        <div className="user-links-list">
          <LinkCard title="По DNS" value={links.dns} />
          <LinkCard title="По IP-адресу" value={links.ip} />
        </div>
        <div className="modal-actions">
          <button className="btn secondary" onClick={onClose}>
            Закрыть
          </button>
        </div>
      </section>
    </div>
  );
}
