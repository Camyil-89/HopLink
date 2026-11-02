import * as AppApi from "../../wailsjs/go/desktop/App";

export const api = () => AppApi;

export const uid = () => crypto.randomUUID();

export const logTimestamp = () => new Date().toLocaleTimeString();

export const logLine = (message: string) => `${logTimestamp()}  ${message}`;
