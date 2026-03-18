import axios from "axios";

const configuredApiBase = (import.meta.env.VITE_API_URL as string | undefined)?.trim();
const API_BASE_URL = configuredApiBase && configuredApiBase.length > 0
  ? configuredApiBase
  : window.location.origin;
const API_PATH_PREFIX = (import.meta.env.VITE_API_PATH_PREFIX as string | undefined) ?? "/api/v1";

function joinBaseAndPath(base: string, path: string): string {
  return `${base.replace(/\/$/, "")}${path.startsWith("/") ? path : `/${path}`}`;
}

const api = axios.create({ baseURL: joinBaseAndPath(API_BASE_URL, API_PATH_PREFIX) });

export const API_ENDPOINT = joinBaseAndPath(API_BASE_URL, API_PATH_PREFIX);

export type Deployment = {
  id: string;
  repository: string;
  branch: string;
  status: string;
  url: string;
  localUrl?: string;
  repoUrl?: string;
  error?: string;
  createdAt: string;
};

export type User = {
  id: string;
  username: string;
  role: string;
  email?: string;
  verified?: boolean;
};

export type DeploymentLog = {
  time: string;
  level: string;
  message: string;
};

let authToken = "";

try {
  const stored = localStorage.getItem("auth-storage");
  if (stored) {
    const parsed = JSON.parse(stored) as { state?: { token?: string } };
    if (parsed?.state?.token) authToken = parsed.state.token;
  }
} catch { /* ignore */ }

export function setToken(token: string) {
  authToken = token;
}

api.interceptors.request.use((config) => {
  if (authToken) config.headers.Authorization = `Bearer ${authToken}`;
  return config;
});

export function getApiErrorMessage(err: unknown): string {
  if (!axios.isAxiosError(err)) return "Request failed";

  const backendError = err.response?.data?.error;
  if (typeof backendError === "string" && backendError.trim().length > 0) return backendError;

  const status = err.response?.status;
  const contentType = String(err.response?.headers?.["content-type"] ?? "").toLowerCase();

  if (!err.response) {
    return `Cannot reach API server at ${API_ENDPOINT}.`;
  }

  if (status === 404 && contentType.includes("text/html")) {
    return `API route is not configured at ${API_ENDPOINT}.`;
  }

  return status ? `Request failed (${status}).` : "Request failed";
}

export async function login(username: string, password: string): Promise<{ token: string; user: User }> {
  const { data } = await api.post("/auth/login", { username, password });
  return { token: data.token, user: data.user as User };
}

export async function signup(email: string, username: string, password: string): Promise<{ message: string; user: User; verificationCode?: string }> {
  const { data } = await api.post("/auth/signup", { email, username, password });
  return { message: data.message, user: data.user as User, verificationCode: data.verification_code };
}

export async function verifyAccount(username: string, code: string): Promise<void> {
  await api.post("/auth/verify", { username, code });
}

export async function forgotPassword(username: string, email: string): Promise<{ resetCode?: string }> {
  const { data } = await api.post("/auth/forgot-password", { username, email });
  return { resetCode: data.reset_code };
}

export async function resetPassword(username: string, code: string, newPassword: string): Promise<void> {
  await api.post("/auth/reset-password", { username, code, newPassword });
}

export async function listDeployments(): Promise<Deployment[]> {
  const { data } = await api.get("/deployments");
  return data.items as Deployment[];
}

export async function createDeployment(repository: string, branch: string, url: string): Promise<Deployment> {
  const { data } = await api.post("/deployments", { repository, branch, url });
  return data as Deployment;
}

export async function deleteDeployment(id: string): Promise<void> {
  await api.delete(`/deployments/${id}`);
}

export async function getDeploymentLogs(id: string): Promise<DeploymentLog[]> {
  const { data } = await api.get(`/deployments/${id}/logs`);
  return data.items as DeploymentLog[];
}

export type DeploymentStatus = {
  id: string;
  status: string;
  url: string;
  localUrl?: string;
  error?: string;
  createdAt: string;
  repository: string;
  branch: string;
};

export async function getDeploymentStatus(id: string): Promise<DeploymentStatus> {
  const { data } = await api.get(`/deployments/${id}/status`);
  return data as DeploymentStatus;
}
