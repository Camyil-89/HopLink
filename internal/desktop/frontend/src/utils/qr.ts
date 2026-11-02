import { create } from "qrcode";

/** QR matrix for SVG rendering (quiet zone added by the caller). */
export function qrMatrix(value: string): boolean[][] {
  let qr;
  try {
    qr = create(value, { errorCorrectionLevel: "L" });
  } catch {
    throw new Error("Ссылка слишком длинная для QR-кода");
  }
  const { size } = qr.modules;
  return Array.from({ length: size }, (_, r) =>
    Array.from({ length: size }, (_, c) => qr.modules.get(r, c) === 1),
  );
}
