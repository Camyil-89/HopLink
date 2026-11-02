import { generatePassword } from "../utils/password";
import { UnsavedChangesModal } from "../modals/UnsavedChangesModal";
import { RouteModal } from "../modals/RouteModal";
import { RenameRouteModal } from "../modals/RenameRouteModal";
import { DeleteRouteModal } from "../modals/DeleteRouteModal";
import { RemoveRouteServiceModal } from "../modals/RemoveRouteServiceModal";
import { RouteJournalModal } from "../modals/RouteJournalModal";
import { ClearMapModal } from "../modals/ClearMapModal";
import { ServerModal } from "../modals/ServerModal";
import { UserModal } from "../modals/UserModal";
import { UserLinksModal } from "../modals/UserLinksModal";
import {
  CancelApplyModal,
  HostApprovalModal,
  ProgressModal,
} from "../modals/OperationModals";
import { api } from "./api";
import type { HoplinkAppModel } from "./useHoplinkApp";

type Props = { app: HoplinkAppModel };

export function AppModals({ app }: Props) {
  const activeRouteName = app.state.routes.find(
    (route) => route.id === app.state.activeRouteId,
  )?.name;

  return (
    <>
      {app.routeForm && (
        <RouteModal
          name={app.routeName}
          onNameChange={app.setRouteName}
          onClose={() => app.setRouteForm(false)}
          onCreate={() => void app.createRoute()}
        />
      )}
      {app.renameRouteTarget && (
        <RenameRouteModal
          name={app.renameRouteName}
          onNameChange={app.setRenameRouteName}
          onClose={() => app.setRenameRouteTarget(null)}
          onSave={() => void app.renameRoute()}
        />
      )}
      {app.deleteRouteTarget && (
        <DeleteRouteModal
          route={app.deleteRouteTarget}
          onCancel={() => app.setDeleteRouteTarget(null)}
          onConfirm={() => void app.deleteRoute(app.deleteRouteTarget!)}
        />
      )}
      {app.journalOpen && (
        <RouteJournalModal
          servers={app.state.deployedServers || []}
          onClose={() => app.setJournalOpen(false)}
          onLoad={(serverID, since, until) =>
            api().GetRouteJournal(
              app.state.activeRouteId,
              serverID,
              since,
              until,
            )
          }
        />
      )}
      {app.clearMapConfirm && (
        <ClearMapModal
          serverCount={app.nodes.length}
          connectionCount={app.edges.length}
          onCancel={() => app.setClearMapConfirm(false)}
          onConfirm={() => {
            app.setNodes([]);
            app.setEdges([]);
            app.setActiveNode(null);
            app.setClearMapConfirm(false);
          }}
        />
      )}
      {app.removeRouteConfirm && (
        <RemoveRouteServiceModal
          routeName={activeRouteName || "Маршрут"}
          servers={app.state.deployedServers || []}
          onCancel={() => app.setRemoveRouteConfirm(false)}
          onConfirm={() => void app.removeRouteService(app.state.activeRouteId)}
        />
      )}
      {app.serverForm && (
        <ServerModal
          draft={app.draftServer}
          editing={!!app.editingServer}
          onChange={app.setDraftServer}
          onClose={() => app.setServerForm(false)}
          onSave={() => void app.addServer()}
        />
      )}
      {app.userForm && (
        <UserModal
          draft={app.draftUser}
          editing={!!app.editingUser}
          onChange={app.setDraftUser}
          onGeneratePassword={() =>
            app.setDraftUser((current) => ({
              ...current,
              password: generatePassword(),
            }))
          }
          onClose={() => app.setUserForm(false)}
          onSave={() => void app.addUser()}
        />
      )}
      {app.userLinks && (
        <UserLinksModal
          name={app.userLinks.name}
          links={app.userLinks}
          onClose={() => app.setUserLinks(null)}
        />
      )}
      {app.unsavedOpen && (
        <UnsavedChangesModal
          onStay={() => {
            app.setUnsavedOpen(false);
            app.setPendingNavigation(null);
          }}
          onDiscard={() => void app.discardAndContinue()}
          onSave={() => void app.saveAndContinue()}
        />
      )}
      {app.progressOpen && (
        <ProgressModal
          title={app.progressTitle}
          status={app.operationStatus}
          logs={app.logs}
          busy={app.busy}
          result={app.operationResult}
          cancelable={app.progressCancelable}
          endRef={app.logEndRef}
          onClose={app.closeProgress}
          onCancel={() => app.setCancelConfirm(true)}
        />
      )}
      {app.hostApproval && (
        <HostApprovalModal
          host={app.hostApproval.host}
          fingerprint={app.hostApproval.fp}
          onCancel={() => {
            app.setResumeApply(false);
            app.setHostApproval(null);
          }}
          onApprove={() => void app.approveHost()}
        />
      )}
      {app.cancelConfirm && (
        <CancelApplyModal
          onContinue={() => app.setCancelConfirm(false)}
          onConfirm={() => void app.requestApplyCancel()}
        />
      )}
      {app.notice && (
        <div
          className={`toast ${app.notice.includes("Ошибка") || app.notice.includes("Error") || app.notice.includes("fingerprint") ? "error" : ""}`}
        >
          <span>{app.notice}</span>
          <button onClick={() => app.setNotice("")}>×</button>
        </div>
      )}
    </>
  );
}
