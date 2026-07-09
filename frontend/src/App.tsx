import { useState, useEffect } from "react";
import { useSystemState } from "@/hooks/useSystemState";
import { useSession } from "@/hooks/useSession";
import { useTelemetry } from "@/hooks/useTelemetry";
import { useVehicleAck } from "@/hooks/useVehicleAck";
import { SafeModeOverlay } from "@/components/SafeModeOverlay";
import { SafetyPanel } from "@/components/SafetyPanel";
import { ConnectionPanel } from "@/components/ConnectionPanel";
import { VideoPanel } from "@/components/VideoPanel";
import { ControlPanel } from "@/components/ControlPanel";
import { InputIndicatorPanel } from "@/components/InputIndicatorPanel";
import { StreamSenderPanel } from "@/components/StreamSenderPanel";
import LoginPanel from "@/components/LoginPanel";
import UserManagementPanel from "@/components/UserManagementPanel";
import { parseTokenRole, listSessions, ActiveSession } from "@/lib/api-client";
import { SessionState } from "@/hooks/useSession";

// ─── Constants ────────────────────────────────────────────────────────────────

const STATE_COLORS: Record<string, string> = {
  IDLE: "bg-gray-500",
  CONNECTING: "bg-blue-500",
  AUTHENTICATED: "bg-blue-400",
  CONNECTED: "bg-green-500",
  DEGRADED: "bg-yellow-500",
  SAFE_MODE: "bg-red-600",
  RECOVERING: "bg-orange-500",
};

const OPERATOR_ROLE_LABEL: Record<string, string> = {
  NO_OPERATOR: "",
  OPERATOR_ASSIGNED: "Assigned",
  ACTIVE_OPERATOR: "Active Operator",
  HANDOVER_PENDING: "Handover…",
  RECOVERING_OPERATOR: "Recovering",
};

// ─── Sub-components ───────────────────────────────────────────────────────────

function SystemStateBadge({ state }: { state: string }) {
  const color = STATE_COLORS[state] ?? "bg-gray-600";
  return (
    <span className={`px-2 py-1 rounded text-white text-sm font-mono ${color}`}>
      {state}
    </span>
  );
}

// AppContent mounts only after login — all data-fetching hooks live here.
// This prevents background polling (useSystemState, useTelemetry, …) during the login screen.
function AppContent({ session }: { session: SessionState }) {
  const state = useSystemState(session.vehicleId, session.token!);
  const telemetry = useTelemetry(session.vehicleId);
  const vehicleAck = useVehicleAck(session.vehicleId);
  const [showSender, setShowSender] = useState(false);
  const [videoLatency, setVideoLatency] = useState<number | null>(null);
  const [showUserMgmt, setShowUserMgmt] = useState(false);
  const [activeSessions, setActiveSessions] = useState<ActiveSession[]>([]);

  const isConnected = state.system === "CONNECTED" || state.system === "DEGRADED";
  const isSafeMode = state.system === "SAFE_MODE";
  const isUnreachable = state.unreachable;
  const tokenRole = parseTokenRole(session.token!);
  const isAdmin = tokenRole === "ADMIN";
  const isObserver = tokenRole === "OBSERVER";
  const canControl = !isObserver && session.role !== 'OBSERVER';

  useEffect(() => {
    const token = session.token
    if (!token) return
    const poll = async () => {
      try { setActiveSessions((await listSessions(token)) ?? []) } catch { /* ignore */ }
    }
    poll()
    const id = setInterval(poll, 3000)
    return () => clearInterval(id)
  }, [session.token])

  // After page reload during an active session: the frontend lost its local context
  // (sessionId/vehicleId) but the server still has the session. Find it via the own
  // operator_id in the activeSessions list (GET /sessions) and restore refs so the
  // SAFE_MODE overlay / resume() can work. Decoupled from isSafeMode (ADR-026) — vehicleId
  // discovery no longer depends on the per-vehicle system state, which itself depends on
  // knowing the vehicleId first.
  const { sessionId: localSessionId, operatorId, restoreFromServerState } = session
  useEffect(() => {
    if (localSessionId || !operatorId) return
    const mine = activeSessions.find((s) => s.operator_id === operatorId)
    if (mine) restoreFromServerState(mine.session_id, mine.vehicle_id, mine.role)
  }, [activeSessions, localSessionId, operatorId, restoreFromServerState])

  // Multiple vehicles can each have their own ACTIVE_OPERATOR at once (ADR-026) —
  // the admin panel must lock all of them, not just one.
  const activeOperatorIds = activeSessions
    .filter((s) => s.role === 'ACTIVE_OPERATOR')
    .map((s) => s.operator_id)

  return (
    <div className="min-h-screen bg-gray-900 text-white flex flex-col">
      {isSafeMode && <SafeModeOverlay onResume={session.resume} />}

      {showUserMgmt && (
        <UserManagementPanel
          token={session.token!}
          currentUsername={session.operatorId ?? ""}
          activeOperatorIds={activeOperatorIds}
          onClose={() => setShowUserMgmt(false)}
        />
      )}

      <header className="bg-gray-800 border-b border-gray-700 px-6 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold tracking-wide">
          AVOC — Teleoperation Control Center
        </h1>
        <div className="flex items-center gap-3">
          {state.operator !== "NO_OPERATOR" && (
            <span className="text-xs font-mono text-gray-400 bg-gray-700 px-2 py-1 rounded">
              {OPERATOR_ROLE_LABEL[state.operator] ?? state.operator}
            </span>
          )}
          {isAdmin && (
            <button
              onClick={() => setShowUserMgmt(true)}
              className="px-3 py-1 rounded text-xs font-semibold bg-gray-700 hover:bg-gray-600 text-gray-300"
            >
              Benutzerverwaltung
            </button>
          )}
          <button
            onClick={() => setShowSender((v) => !v)}
            className={`px-3 py-1 rounded text-xs font-semibold transition-colors ${
              showSender
                ? "bg-indigo-700 text-white"
                : "bg-gray-700 hover:bg-gray-600 text-gray-300"
            }`}
          >
            ⏺ Senden
          </button>
          <button
            onClick={session.disconnect}
            disabled={session.role === 'ACTIVE_OPERATOR' && !!session.sessionId}
            title={session.role === 'ACTIVE_OPERATOR' && !!session.sessionId
              ? 'Session erst beenden bevor du dich abmeldest'
              : undefined}
            className="px-3 py-1 rounded text-xs font-semibold bg-gray-700 hover:bg-gray-600 text-gray-300 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Abmelden
          </button>
          <SystemStateBadge state={state.system} />
        </div>
      </header>

      {isUnreachable && (
        <div className="bg-red-950 border-b border-red-700 px-6 py-2 text-red-300 text-sm text-center font-semibold">
          ✕ Backend nicht erreichbar — Steuerung blockiert. Verbindung wird wiederhergestellt…
        </div>
      )}

      {state.system === "DEGRADED" && (
        <div className="bg-yellow-900/50 border-b border-yellow-700 px-6 py-2 text-yellow-300 text-sm text-center">
          ⚠ DEGRADED — Video oder Telemetrie ausgefallen. Steuerung weiterhin möglich.
        </div>
      )}

      <main className="flex-1 grid grid-cols-3 grid-rows-2 gap-4 p-4 min-h-0">
        <VideoPanel
          sessionId={session.sessionId}
          vehicleId={session.vehicleId ?? ""}
          token={session.token}
          enabled={isConnected}
          onVideoLatency={setVideoLatency}
        />
        <SafetyPanel
          systemState={state.system}
          sessionId={session.sessionId}
          vehicleId={session.vehicleId}
          wsClient={session.wsClient}
          token={session.token}
        />
        <ConnectionPanel
          systemState={state.system}
          operatorState={state.operator}
          sessionId={session.sessionId}
          vehicleId={session.vehicleId}
          role={session.role}
          latency={session.latency}
          videoLatency={videoLatency}
          telemetry={telemetry}
          activeSessions={activeSessions}
          onStartSession={isObserver ? undefined : session.startSession}
          onJoinSession={isObserver ? session.startSession : undefined}
          onEndSession={session.endSession}
        />
      </main>

      {showSender && <StreamSenderPanel />}

      {canControl && (
        <footer className="bg-gray-800 border-t border-gray-700">
          <ControlPanel
            wsClient={session.wsClient}
            sessionId={session.sessionId}
            vehicleId={session.vehicleId}
            enabled={isConnected && !isUnreachable}
          />
          <InputIndicatorPanel telemetry={telemetry} ack={vehicleAck} />
        </footer>
      )}
    </div>
  );
}

// ─── Root ─────────────────────────────────────────────────────────────────────

// App only manages the session token. No data-fetching hooks run here,
// so the login screen is completely idle — no backend polling before login.
export default function App() {
  const session = useSession();

  if (!session.token) {
    return <LoginPanel onLogin={session.connect} />;
  }

  return <AppContent session={session} />;
}
