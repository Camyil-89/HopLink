import { Map, Server as ServerIcon, Settings2, Trash2 } from "lucide-react";
import type { NavigationItem } from "../components/AppChrome";

export const appNavigation: NavigationItem[] = [
  { id: "routes", label: "Маршруты", icon: Map },
  { id: "servers", label: "Серверы", icon: ServerIcon },
  { id: "remove", label: "Удалить HopLink", icon: Trash2 },
  { id: "desktop-settings", label: "Настройки", icon: Settings2 },
];
