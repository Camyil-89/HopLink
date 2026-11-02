# React frontend

- `src/app/App.tsx`: vault gate and shell composition.
- `src/app/useHoplinkApp.ts`: application state, Wails calls, and handlers.
- `src/app/AppMain.tsx`, `AppModals.tsx`, `VaultScreen.tsx`: main UI sections.
- `src/tabs/`: top-level screens and route tabs.
- `src/components/`: reusable UI elements, including the map canvas and nodes.
- `src/modals/`: forms, confirmations, and operation progress dialogs.
- `src/types/`: frontend types; `src/utils/`: shared frontend logic.
- `src/assets/`: images and CSS.
- `wailsjs/`: generated Go bindings and Wails runtime; regenerate via the Wails build rather than editing generated files.

Route editing has separate map, settings, and users tabs. Maintain unsaved-change handling when navigating away. Keep route-specific user state and connection links associated with the selected route. Avoid adding new monolithic screen or modal implementations to `src/app/App.tsx`.

From the repository root, run `node scripts/build-desktop.mjs` to bundle the frontend and rebuild `deploy/HopLink.exe` after UI changes.
