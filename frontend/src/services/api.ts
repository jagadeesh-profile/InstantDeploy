import axios from "axios";

// Use VITE_API_URL if set, otherwise use relative URLs (works with nginx proxy)
const API_BASE_URL = import.meta.env.VITE_API_URL || "";

const api = axios.create({ baseURL: `${API_BASE_URL}/api/v1` });

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
