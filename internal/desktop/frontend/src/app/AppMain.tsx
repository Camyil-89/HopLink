import { Sidebar, Topbar, RouteWorkspaceHeader } from "../components/AppChrome";
import { RoutesTab } from "../tabs/RoutesTab";
import { MapTab } from "../tabs/MapTab";
import { SettingsTab } from "../tabs/SettingsTab";
import { UsersTab } from "../tabs/UsersTab";
import { ServersTab } from "../tabs/ServersTab";
import { RemoveTab } from "../tabs/RemoveTab";
import { DesktopSettingsTab } from "../tabs/DesktopSettingsTab";
import { api } from "./api";
import type { HoplinkAppModel } from "./useHoplinkApp";

type Props = { app: HoplinkAppModel };

export function AppMain({ app }: Props) {
  const activeRouteName = app.state.routes.find(
    (route) => route.id === app.state.activeRouteId,
  )?.name;

  const mainClass =
    app.page === "routes" && app.routeChosen && app.routeTab === "map"
      ? "route-map-main"
      : app.page === "routes" && !app.routeChosen
        ? "route-list-main"
        : "";

  return (
    <div
      className={`app-shell min-h-screen antialiased ${app.collapsed ? "is-collapsed" : ""}`}
    >
      <Sidebar
        collapsed={app.collapsed}
        currentPage={app.page}
        items={app.nav}
        onToggle={() => app.setCollapsed((value) => !value)}
        onNavigate={app.navigateTo}
      />
      <main className={`main ${mainClass}`}>
        <Topbar
          section={
            app.nav.find((item) => item.id === app.page)?.label || "Маршруты"
          }
          routeName={
            app.page === "routes" && app.routeChosen
              ? activeRouteName
              : undefined
          }
        />
        {app.page === "routes" && !app.routeChosen && (
          <RoutesTab
            routes={app.state.routes}
            statuses={app.routeStatuses}
            checking={app.checkingRoutes}
            onCheck={() => void app.checkRouteServices()}
            onSelect={app.selectRoute}
            onCreate={() => {
              app.setRouteName("");
              app.setRouteForm(true);
            }}
            onRename={(route) => {
              app.setRenameRouteTarget(route);
              app.setRenameRouteName(route.name);
            }}
            onDelete={app.setDeleteRouteTarget}
          />
        )}
        {app.page === "routes" && app.routeChosen && (
          <RouteWorkspaceHeader
            routeName={activeRouteName || ""}
            tab={app.routeTab}
            dirty={app.routeDirty}
            installedServers={app.state.deployedServers?.length ?? 0}
            busy={app.busy}
            onBack={() =>
              app.withUnsavedGuard(() => {
                app.setRouteChosen(false);
                app.setPage("routes");
              })
            }
            onTabChange={(tab) =>
              app.withUnsavedGuard(() => app.setRouteTab(tab))
            }
            onSave={() => void app.saveRoute()}
            onRemoveService={() =>
              app.withUnsavedGuard(() => app.setRemoveRouteConfirm(true))
            }
          />
        )}
        {app.page === "routes" && app.routeChosen && app.routeTab === "map" && (
          <MapTab
            state={app.state}
            nodes={app.nodes}
            flowNodes={app.flowNodes}
            edges={app.flowEdges}
            servers={app.cardServers}
            search={app.search}
            dirty={app.routeDirty}
            busy={app.busy}
            fitRevision={app.fitRevision}
            onSearch={app.setSearch}
            onNavigateServers={() => app.navigateTo("servers")}
            onAddServer={(server) => {
              const x = Math.round((100 + Math.random() * 300) / 20) * 20;
              const y = Math.round((100 + Math.random() * 200) / 20) * 20;
              app.setNodes((current) =>
                current.some((node) => node.id === server.id)
                  ? current
                  : [
                      ...current,
                      {
                        id: server.id,
                        position: { x, y },
                        data: { server },
                        type: "server",
                      },
                    ],
              );
            }}
            onDrop={app.onDrop}
            onClear={() => app.setClearMapConfirm(true)}
            onNodeDragStop={(node) => {
              app.setNodes((current) =>
                current.map((item) =>
                  item.id === node.id
                    ? {
                        ...item,
                        position: {
                          x: Math.round(node.position.x / 20) * 20,
                          y: Math.round(node.position.y / 20) * 20,
                        },
                      }
                    : item,
                ),
              );
              app.setFitRevision((value) => value + 1);
            }}
            onNodesChange={app.onNodesChange}
            onEdgesChange={app.onEdgesChange}
            onConnect={app.onConnect}
            canConnect={app.canConnect}
            onEdgeClick={(edge) =>
              app.setEdges((current) =>
                current.map((item) => ({
                  ...item,
                  selected: item.id === edge.id,
                })),
              )
            }
            onNodeClick={app.onNodeClick}
            onPaneClick={() => app.setActiveNode(null)}
            onSave={() => void app.saveRoute()}
            onApply={() => void app.apply()}
          />
        )}
        {app.page === "routes" &&
          app.routeChosen &&
          app.routeTab === "settings" && (
            <SettingsTab
              state={app.state}
              showSecrets={app.showSecrets}
              onChange={(patch) =>
                app.setState((current) => ({ ...current, ...patch }))
              }
              onToggleSecrets={() => app.setShowSecrets((value) => !value)}
              onGetJournal={() => app.setJournalOpen(true)}
            />
          )}
        {app.page === "routes" &&
          app.routeChosen &&
          app.routeTab === "users" && (
            <UsersTab
              state={app.state}
              onAdd={() => {
                app.setEditingUser(null);
                app.setDraftUser({ name: "", password: "" });
                app.setUserForm(true);
              }}
              onEdit={(user) => {
                app.setEditingUser(user.id);
                app.setDraftUser({ name: user.name, password: user.password });
                app.setUserForm(true);
              }}
              onDelete={(id) => void app.deleteUser(id)}
              onShowLinks={(user) => {
                void api()
                  .GetUserLinkVariants(user.id)
                  .then((result) =>
                    app.setUserLinks({
                      name: user.name,
                      dns: result.dns || "",
                      ip: result.ip || "",
                    }),
                  )
                  .catch((error: unknown) => app.setNotice(String(error)));
              }}
              onToggle={(id, active) => void app.toggleUser(id, active)}
            />
          )}
        {app.page === "servers" && (
          <ServersTab
            state={app.state}
            busy={app.busy}
            onAdd={app.openAddServer}
            onEdit={app.openEditServer}
            onDelete={(id) => void app.deleteServer(id)}
          />
        )}
        {app.page === "desktop-settings" && (
          <DesktopSettingsTab
            installSource={
              app.state.installSource === "source" ? "source" : "github"
            }
            onInstallSourceChange={async (source) => {
              await api().SetInstallSource(source);
              app.setState((current) => ({
                ...current,
                installSource: source,
              }));
            }}
          />
        )}
        {app.page === "remove" && app.routeChosen && (
          <RemoveTab
            state={app.state}
            selected={app.selected}
            busy={app.busy}
            onToggle={(id) =>
              app.setSelected((current) =>
                current.includes(id)
                  ? current.filter((item) => item !== id)
                  : [...current, id],
              )
            }
            onToggleAll={() =>
              app.setSelected(
                app.selected.length === app.state.servers.length
                  ? []
                  : app.state.servers.map((server) => server.id),
              )
            }
            onRemove={() => void app.removeSelected()}
          />
        )}
      </main>
    </div>
  );
}
