import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Rocket, Plus, X } from "lucide-react";
import toast from "react-hot-toast";
import DeploymentCard from "../components/DeploymentCard";
import { useDeployments } from "../hooks/useDeployments";
import { getDeploymentLogs, type DeploymentLog } from "../services/api";
import { websocketService, type WebSocketMessage } from "../services/websocket";
import { useAuthStore } from "../store/authStore";

export default function DashboardPage() {
  const { items, loading, error, deploy, refresh, remove, applyStatusUpdate } = useDeployments();
  const { user } = useAuthStore();

  const [showForm, setShowForm] = useState(false);
  const [repository, setRepository] = useState("https://github.com/vercel/serve");
  const [branch, setBranch] = useState("main");
  const [deploymentUrl, setDeploymentUrl] = useState("https://demo.instantdeploy.app");
  const [deploying, setDeploying] = useState(false);
  const [selectedDeploymentId, setSelectedDeploymentId] = useState<string | null>(null);
  const [logs, setLogs] = useState<DeploymentLog[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const logsRequestSeq = useRef(0);

  // Load deployments on mount
  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Poll faster while there are active builds
  useEffect(() => {
    const hasActiveBuild = items.some((d) => ["queued", "building", "cloning", "starting"].includes(d.status));
    if (!hasActiveBuild) {
      return;
    }
    const timer = setInterval(() => {
      void refresh();
    }, 2000);
    return () => clearInterval(timer);
  }, [items, refresh]);

  // Poll selected deployment logs
  useEffect(() => {
    if (!selectedDeploymentId) return;

    let isMounted = true;
    const fetchLogs = async () => {
      const seq = ++logsRequestSeq.current;
      setLogsLoading(true);
      try {
        const data = await getDeploymentLogs(selectedDeploymentId);
        if (isMounted && seq === logsRequestSeq.current) setLogs(data);
      } catch {
        if (isMounted && seq === logsRequestSeq.current) setLogs([]);
      } finally {
        if (isMounted && seq === logsRequestSeq.current) setLogsLoading(false);
      }
    };

    void fetchLogs();
    const timer = setInterval(() => {
      void fetchLogs();
    }, 2000);

    return () => {
      isMounted = false;
      clearInterval(timer);
    };
  }, [selectedDeploymentId]);

  // Realtime websocket updates (with polling still active as fallback).
  useEffect(() => {
    if (!user?.id) return;

    websocketService.connect(user.id);

    const unsubscribeStatus = websocketService.on("deployment_status", (message: WebSocketMessage) => {
      const id = String(message.payload.deployment_id ?? "");
      const status = String(message.payload.status ?? "");
      if (id && status) {
        applyStatusUpdate(id, status);
      } else {
        void refresh(); // fallback for malformed events
      }
    });

    const unsubscribeLog = websocketService.on("deployment_log", (message: WebSocketMessage) => {
      if (!selectedDeploymentId) return;

      const deploymentId = String(message.payload.deployment_id ?? "");
      if (deploymentId !== selectedDeploymentId) return;

      const level = String(message.payload.level ?? "info");
      const text = String(message.payload.message ?? "");
      setLogs((prev) => [
        ...prev,
        {
          time: message.timestamp ?? new Date().toISOString(),
          level,
          message: text,
        },
      ]);
    });

    return () => {
      unsubscribeStatus();
      unsubscribeLog();
    };
  }, [applyStatusUpdate, refresh, selectedDeploymentId, user?.id]);

  useEffect(() => {
    return () => {
      websocketService.disconnect();
    };
  }, []);

  const stats = useMemo(
    () => ({
      total: items.length,
      running: items.filter((d) => d.status === "running").length,
      building: items.filter((d) => d.status === "building").length,
      failed: items.filter((d) => d.status === "failed").length,
    }),
    [items]
  );

  async function handleDeploy(e: FormEvent) {
    e.preventDefault();
    setDeploying(true);
    const ok = await deploy(repository, branch, deploymentUrl);
    setDeploying(false);
    if (ok) {
      toast.success("Deployment created!");
      setShowForm(false);
    } else {
      toast.error(error ?? "Deployment failed");
    }
  }

  async function handleDelete(id: string) {
    const ok = await remove(id);
    if (ok) {
      toast.success("Deployment deleted");
      if (selectedDeploymentId === id) {
        setSelectedDeploymentId(null);
        setLogs([]);
      }
      return;
    }
    toast.error("Failed to delete deployment");
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 mb-1">Dashboard</h1>
          <p className="text-gray-500">
            Welcome back,{" "}
            <span className="font-medium text-gray-700">{user?.username}</span>! Manage
            your deployments below.
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

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        {([
          { label: "Total", value: stats.total, color: "text-primary-600" },
          { label: "Running", value: stats.running, color: "text-green-600" },
          { label: "Building", value: stats.building, color: "text-yellow-500" },
          { label: "Failed", value: stats.failed, color: "text-red-600" },
        ] as const).map(({ label, value, color }) => (
          <div
            key={label}
            className="bg-white rounded-xl border border-gray-100 shadow-sm p-5"
          >
            <p className="text-sm text-gray-500 mb-1">{label}</p>
            <p className={`text-3xl font-bold ${color}`}>{value}</p>
          </div>
        ))}
      </div>

      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-gray-900">Deployments</h2>
          <button
            onClick={() => void refresh()}
            className="text-sm text-primary-600 hover:text-primary-700 font-medium"
          >
            Refresh
          </button>
        </div>

        {loading ? (
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
                onViewLogs={(id) => {
                  setLogs([]);
                  setLogsLoading(true);
                  setSelectedDeploymentId(id);
                }}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}
        {error ? <p className="text-red-600 text-sm mt-4">{error}</p> : null}
      </div>

      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-6 mt-6">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold text-gray-900">Backend Logs</h2>
          <span className="text-xs text-gray-500">
            {selectedDeploymentId ? `Deployment: ${selectedDeploymentId}` : "Select a deployment"}
          </span>
        </div>
        {!selectedDeploymentId ? (
          <p className="text-sm text-gray-500">Click "Logs" on a deployment card to view backend activity.</p>
        ) : logsLoading && logs.length === 0 ? (
          <p className="text-sm text-gray-500">Loading logs...</p>
        ) : logs.length === 0 ? (
          <p className="text-sm text-gray-500">No logs yet.</p>
        ) : (
          <div className="bg-gray-900 text-gray-100 rounded-lg p-4 max-h-64 overflow-y-auto font-mono text-xs space-y-1">
            {logs.map((l, idx) => (
              <div key={`${l.time}-${idx}`}>
                <span className="text-gray-400">[{new Date(l.time).toLocaleTimeString()}]</span>{" "}
                <span className="uppercase text-blue-300">{l.level}</span>{" "}
                <span>{l.message}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {showForm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-2xl p-8 w-full max-w-md">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-semibold text-gray-900">New Deployment</h2>
              <button
                onClick={() => setShowForm(false)}
                aria-label="Close deployment form"
                title="Close"
                className="text-gray-400 hover:text-gray-600"
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
                  {deploying ? "Starting Build..." : "Deploy Now"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
