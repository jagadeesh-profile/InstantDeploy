import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Plus, Rocket, X } from "lucide-react";
import toast from "react-hot-toast";
import DeploymentCard from "../components/DeploymentCard";
import { useDeployments } from "../hooks/useDeployments";
import { getDeploymentLogs, type DeploymentLog } from "../services/api";
import { websocketService, type WebSocketMessage } from "../services/websocket";
import { useAuthStore } from "../store/authStore";

const ACTIVE_STATUSES = new Set(["queued", "cloning", "building", "starting"]);

export default function DashboardPage() {
  const { items, loading, error, deploy, refresh, remove, applyStatusUpdate } = useDeployments();
  const { user, token } = useAuthStore();

  const [showForm, setShowForm]             = useState(false);
  const [repository, setRepository]         = useState("octocat/Hello-World");
  const [branch, setBranch]                 = useState("main");
  const [deploymentUrl, setDeploymentUrl]   = useState("");
  const [deploying, setDeploying]           = useState(false);

  const [selectedId, setSelectedId]         = useState<string | null>(null);
  const [logs, setLogs]                     = useState<DeploymentLog[]>([]);
  const [logsLoading, setLogsLoading]       = useState(false);
  const logsSeq                             = useRef(0);
  const logsEndRef                          = useRef<HTMLDivElement>(null);

  // ---- initial load ----
  useEffect(() => { void refresh(); }, [refresh]);

  // ---- poll while builds are active ----
  useEffect(() => {
    const hasActive = items.some((d) => ACTIVE_STATUSES.has(d.status));
    if (!hasActive) return;
    const t = setInterval(() => void refresh(), 2500);
    return () => clearInterval(t);
  }, [items, refresh]);

  // ---- poll selected deployment logs ----
  const fetchLogs = useCallback(async (id: string) => {
    const seq = ++logsSeq.current;
    setLogsLoading(true);
    try {
      const data = await getDeploymentLogs(id);
      if (seq === logsSeq.current) setLogs(data);
    } catch {
      if (seq === logsSeq.current) setLogs([]);
    } finally {
      if (seq === logsSeq.current) setLogsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!selectedId) return;
    void fetchLogs(selectedId);
    const t = setInterval(() => void fetchLogs(selectedId), 2500);
    return () => clearInterval(t);
  }, [selectedId, fetchLogs]);

  // ---- auto-scroll logs ----
  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  // ---- WebSocket realtime updates ----
  useEffect(() => {
    if (!user?.id || !token) return;
    websocketService.connect(user.id, token);

    const unsubStatus = websocketService.on("deployment_status", (msg: WebSocketMessage) => {
      const id     = String(msg.payload.deployment_id ?? "");
      const status = String(msg.payload.status ?? "");
      if (id && status) applyStatusUpdate(id, status);
      else void refresh();
    });

    const unsubLog = websocketService.on("deployment_log", (msg: WebSocketMessage) => {
      if (!selectedId) return;
      if (String(msg.payload.deployment_id ?? "") !== selectedId) return;
      setLogs((prev) => [
        ...prev,
        {
          time:    String(msg.timestamp ?? new Date().toISOString()),
          level:   String(msg.payload.level   ?? "info"),
          message: String(msg.payload.message ?? ""),
        },
      ]);
    });

    return () => { unsubStatus(); unsubLog(); };
  }, [applyStatusUpdate, refresh, selectedId, token, user?.id]);

  useEffect(() => () => websocketService.disconnect(), []);

  useEffect(() => {
    if (!showForm) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setShowForm(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [showForm]);

  // ---- derived stats ----
  const stats = useMemo(() => ({
    total:    items.length,
    running:  items.filter((d) => d.status === "running").length,
    building: items.filter((d) => ACTIVE_STATUSES.has(d.status)).length,
    failed:   items.filter((d) => d.status === "failed").length,
  }), [items]);

  // ---- handlers ----
  async function handleDeploy(e: FormEvent) {
    e.preventDefault();
    setDeploying(true);
    const ok = await deploy(repository, branch, deploymentUrl);
    setDeploying(false);
    if (ok) {
      toast.success("Deployment queued!");
      setShowForm(false);
    } else {
      toast.error(error ?? "Deployment failed");
    }
  }

  async function handleDelete(id: string) {
    const ok = await remove(id);
    if (ok) {
      toast.success("Deployment deleted");
      if (selectedId === id) { setSelectedId(null); setLogs([]); }
    } else {
      toast.error("Failed to delete deployment");
    }
  }

  function handleViewLogs(id: string) {
    setLogs([]);
    setLogsLoading(true);
    setSelectedId(id);
  }

  const logLevelColor: Record<string, string> = {
    error: "text-red-400",
    warn:  "text-yellow-400",
    info:  "text-blue-300",
  };

  return (
    <div className="space-y-5 md:space-y-8">
      {/* Header */}
      <div className="surface-glass p-4 sm:p-5 md:p-6 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 reveal-up">
        <div>
          <h1 className="font-display text-3xl md:text-4xl font-semibold text-slate-900 mb-1">Dashboard</h1>
          <p className="text-slate-600">
            Welcome back, <span className="font-semibold text-slate-800">{user?.username}</span>.
          </p>
        </div>
        <button
          onClick={() => setShowForm(true)}
          className="focus-ring inline-flex min-h-11 items-center justify-center gap-2 bg-gradient-to-r from-primary-600 to-secondary-500 text-white px-5 py-2.5 rounded-xl font-semibold hover:brightness-105 transition shadow-glow"
        >
          <Plus size={18} />
          New Deployment
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4 reveal-up reveal-delay-1">
        {[
          { label: "Total", value: stats.total, color: "text-primary-700" },
          { label: "Running", value: stats.running, color: "text-emerald-700" },
          { label: "Building", value: stats.building, color: "text-amber-600" },
          { label: "Failed", value: stats.failed, color: "text-red-700" },
        ].map(({ label, value, color }) => (
          <div key={label} className="surface-card p-4 sm:p-5">
            <p className="text-sm text-slate-500 mb-1">{label}</p>
            <p className={`font-display text-3xl font-semibold ${color}`}>{value}</p>
          </div>
        ))}
      </div>

      {/* Deployments list */}
      <div className="surface-card p-4 sm:p-5 md:p-6 reveal-up reveal-delay-2">
        <div className="flex items-center justify-between mb-5">
          <h2 className="font-display text-xl font-semibold text-slate-900">Deployments</h2>
          <button
            onClick={() => void refresh()}
            className="focus-ring rounded-lg px-2 py-1 text-sm text-primary-700 hover:text-primary-800 font-semibold"
          >
            Refresh
          </button>
        </div>

        {loading && items.length === 0 ? (
          <div className="flex justify-center py-12">
            <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-500" />
          </div>
        ) : items.length === 0 ? (
          <div className="text-center py-12">
            <Rocket size={48} className="mx-auto text-slate-300 mb-4" />
            <p className="text-slate-600 mb-1">No deployments yet</p>
            <p className="text-sm text-slate-500">Click "New Deployment" to get started</p>
          </div>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {items.map((deployment) => (
              <DeploymentCard
                key={deployment.id}
                deployment={deployment}
                onViewLogs={handleViewLogs}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}

        {error && <p className="text-red-700 text-sm mt-4">{error}</p>}
      </div>

      {/* Logs panel */}
      <div className="surface-card p-4 sm:p-5 md:p-6 reveal-up">
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-display text-xl font-semibold text-slate-900">Build Logs</h2>
          <div className="flex items-center gap-3">
            {selectedId && (
              <button
                onClick={() => {
                  setSelectedId(null);
                  setLogs([]);
                }}
                className="focus-ring rounded-lg px-2 py-1 text-xs text-slate-500 hover:text-slate-700"
              >
                Clear
              </button>
            )}
            <span className="text-xs text-slate-500">
              {selectedId ? `ID: ${selectedId.slice(0, 16)}...` : "Select a deployment"}
            </span>
          </div>
        </div>

        {!selectedId ? (
          <p className="text-sm text-slate-500">Click "Logs" on a deployment card to view build output.</p>
        ) : logsLoading && logs.length === 0 ? (
          <div className="flex items-center gap-2 text-sm text-slate-500">
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary-400" />
            Loading...
          </div>
        ) : logs.length === 0 ? (
          <p className="text-sm text-slate-500">No logs yet.</p>
        ) : (
          <div className="bg-slate-950 rounded-xl p-4 max-h-72 overflow-y-auto font-mono text-xs space-y-0.5 border border-slate-800" aria-live="polite">
            {logs.map((l, idx) => (
              <div key={`${l.time}-${idx}`} className="flex gap-2 leading-5">
                <span className="text-gray-500 shrink-0">
                  {new Date(l.time).toLocaleTimeString()}
                </span>
                <span className={`uppercase shrink-0 w-8 ${logLevelColor[l.level] ?? "text-gray-400"}`}>
                  {l.level.slice(0, 4)}
                </span>
                <span className="text-gray-200 break-all">{l.message}</span>
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>
        )}
      </div>

      {/* New deployment modal */}
      {showForm && (
        <div className="fixed inset-0 bg-slate-950/45 backdrop-blur-sm flex items-center justify-center z-50 p-4" role="dialog" aria-modal="true" aria-labelledby="new-deployment-title">
          <div className="surface-card p-5 sm:p-6 md:p-8 w-full max-w-md reveal-up">
            <div className="flex items-center justify-between mb-6">
              <h2 id="new-deployment-title" className="font-display text-2xl font-semibold text-slate-900">New Deployment</h2>
              <button
                onClick={() => setShowForm(false)}
                aria-label="Close"
                className="focus-ring rounded-lg text-slate-400 hover:text-slate-600 transition"
              >
                <X size={22} />
              </button>
            </div>

            <form onSubmit={handleDeploy} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Repository</label>
                <input
                  value={repository}
                  onChange={(e) => setRepository(e.target.value)}
                  placeholder="owner/repo or https://github.com/owner/repo"
                  required
                  className="focus-ring w-full min-h-11 px-4 py-3 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-300"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Branch</label>
                <input
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  placeholder="main"
                  className="focus-ring w-full min-h-11 px-4 py-3 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-300"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">
                  Custom URL <span className="text-slate-400 font-normal">(optional)</span>
                </label>
                <input
                  value={deploymentUrl}
                  onChange={(e) => setDeploymentUrl(e.target.value)}
                  placeholder="https://myapp.example.com"
                  className="focus-ring w-full min-h-11 px-4 py-3 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-300"
                />
              </div>

              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowForm(false)}
                  className="focus-ring flex-1 min-h-11 py-3 border border-slate-200 rounded-xl text-slate-700 hover:bg-slate-50 transition font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={deploying}
                  className="focus-ring flex-1 min-h-11 bg-gradient-to-r from-primary-600 to-secondary-500 text-white py-3 rounded-xl font-semibold hover:brightness-105 transition disabled:opacity-60"
                >
                  {deploying ? "Queuing..." : "Deploy Now"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
