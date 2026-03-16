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
  const { user } = useAuthStore();

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
    if (!user?.id) return;
    websocketService.connect(user.id);

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
  }, [applyStatusUpdate, refresh, selectedId, user?.id]);

  useEffect(() => () => websocketService.disconnect(), []);

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
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 mb-1">Dashboard</h1>
          <p className="text-gray-500">
            Welcome back,{" "}
            <span className="font-medium text-gray-700">{user?.username}</span>!
          </p>
        </div>
        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 bg-gradient-to-r from-primary-500 to-secondary-500 text-white px-5 py-2.5 rounded-xl font-medium hover:opacity-90 transition"
        >
          <Plus size={18} />
          New Deployment
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        {[
          { label: "Total",    value: stats.total,    color: "text-primary-600" },
          { label: "Running",  value: stats.running,  color: "text-green-600"   },
          { label: "Building", value: stats.building, color: "text-yellow-500"  },
          { label: "Failed",   value: stats.failed,   color: "text-red-600"     },
        ].map(({ label, value, color }) => (
          <div key={label} className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
            <p className="text-sm text-gray-500 mb-1">{label}</p>
            <p className={`text-3xl font-bold ${color}`}>{value}</p>
          </div>
        ))}
      </div>

      {/* Deployments list */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-6 mb-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-gray-900">Deployments</h2>
          <button
            onClick={() => void refresh()}
            className="text-sm text-primary-600 hover:text-primary-700 font-medium"
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
            <Rocket size={48} className="mx-auto text-gray-300 mb-4" />
            <p className="text-gray-500 mb-1">No deployments yet</p>
            <p className="text-sm text-gray-400">Click "New Deployment" to get started</p>
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

        {error && <p className="text-red-600 text-sm mt-4">{error}</p>}
      </div>

      {/* Logs panel */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-6">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold text-gray-900">Build Logs</h2>
          <div className="flex items-center gap-3">
            {selectedId && (
              <button
                onClick={() => { setSelectedId(null); setLogs([]); }}
                className="text-xs text-gray-400 hover:text-gray-600"
              >
                Clear
              </button>
            )}
            <span className="text-xs text-gray-500">
              {selectedId ? `ID: ${selectedId.slice(0, 16)}…` : "Select a deployment"}
            </span>
          </div>
        </div>

        {!selectedId ? (
          <p className="text-sm text-gray-400">Click "Logs" on a deployment card to view build output.</p>
        ) : logsLoading && logs.length === 0 ? (
          <div className="flex items-center gap-2 text-sm text-gray-500">
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary-400" />
            Loading…
          </div>
        ) : logs.length === 0 ? (
          <p className="text-sm text-gray-400">No logs yet.</p>
        ) : (
          <div className="bg-gray-950 rounded-lg p-4 max-h-72 overflow-y-auto font-mono text-xs space-y-0.5">
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
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-2xl p-8 w-full max-w-md">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-semibold text-gray-900">New Deployment</h2>
              <button
                onClick={() => setShowForm(false)}
                aria-label="Close"
                className="text-gray-400 hover:text-gray-600 transition"
              >
                <X size={22} />
              </button>
            </div>

            <form onSubmit={handleDeploy} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Repository
                </label>
                <input
                  value={repository}
                  onChange={(e) => setRepository(e.target.value)}
                  placeholder="owner/repo or https://github.com/owner/repo"
                  required
                  className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Branch
                </label>
                <input
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  placeholder="main"
                  className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Custom URL <span className="text-gray-400 font-normal">(optional)</span>
                </label>
                <input
                  value={deploymentUrl}
                  onChange={(e) => setDeploymentUrl(e.target.value)}
                  placeholder="https://myapp.example.com"
                  className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400"
                />
              </div>

              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowForm(false)}
                  className="flex-1 py-3 border border-gray-200 rounded-xl text-gray-700 hover:bg-gray-50 transition font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={deploying}
                  className="flex-1 bg-gradient-to-r from-primary-500 to-secondary-500 text-white py-3 rounded-xl font-semibold hover:opacity-90 transition disabled:opacity-60"
                >
                  {deploying ? "Queuing…" : "Deploy Now"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
