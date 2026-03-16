import { FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Rocket } from "lucide-react";
import toast from "react-hot-toast";
import {
  forgotPassword,
  login,
  resetPassword,
  setToken,
  signup,
  verifyAccount,
} from "../services/api";
import { useAuthStore } from "../store/authStore";

export default function LoginPage() {
  const navigate = useNavigate();
  const { setAuth, isAuthenticated } = useAuthStore();

  const [mode, setMode] = useState<"login" | "signup" | "verify" | "forgot" | "reset">("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("demo");
  const [password, setPassword] = useState("demo123");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [verificationCode, setVerificationCode] = useState("");
  const [resetCode, setResetCode] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (isAuthenticated) navigate("/");
  }, [isAuthenticated, navigate]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (loading) return;
    setLoading(true);
    try {
      if (mode === "signup") {
        if (password !== confirmPassword) {
          toast.error("Passwords do not match");
          setLoading(false);
          return;
        }
        const signupResp = await signup(email, username, password);
        if (signupResp.verificationCode) {
          setVerificationCode(signupResp.verificationCode);
          toast.success(`Account created. Use verification code: ${signupResp.verificationCode}`);
        } else {
          toast.success("Account created. Check your verification code.");
        }
        setMode("verify");
        setLoading(false);
        return;
      }

      if (mode === "verify") {
        await verifyAccount(username, verificationCode);
        toast.success("Account verified. Please login.");
        setMode("login");
        setLoading(false);
        return;
      }

      if (mode === "forgot") {
        const res = await forgotPassword(username, email);
        if (res.resetCode) {
          setResetCode(res.resetCode);
          toast.success(`Reset code: ${res.resetCode}`);
        } else {
          toast.success("Reset code generated");
        }
        setMode("reset");
        setLoading(false);
        return;
      }

      if (mode === "reset") {
        await resetPassword(username, resetCode, newPassword);
        toast.success("Password reset successful. Please login.");
        setMode("login");
        setLoading(false);
        return;
      }

      const { token, user } = await login(username, password);
      setToken(token);
      setAuth(user, token);
      toast.success("Welcome back!");
      navigate("/");
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } };
      const msg =
        axiosErr?.response?.data?.error ??
        (mode === "login" ? "Login failed" : "Signup failed");
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-500 to-secondary-500 flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-2xl p-8 max-w-md w-full">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="flex items-center justify-center gap-2 mb-3">
            <Rocket className="text-primary-600" size={32} />
            <h1 className="text-3xl font-bold text-gray-900">InstantDeploy</h1>
          </div>
          <p className="text-gray-500 text-sm">
            Deploy mobile runtimes and services from your repositories in seconds.
          </p>
        </div>

        {/* Mode switcher */}
        <div className="flex bg-gray-100 rounded-xl p-1 mb-6">
          <button
            type="button"
            onClick={() => setMode("login")}
            className={`flex-1 py-2 rounded-lg text-sm font-medium transition ${
              mode === "login"
                ? "bg-white text-gray-900 shadow-sm"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            Login
          </button>
          <button
            type="button"
            onClick={() => setMode("signup")}
            className={`flex-1 py-2 rounded-lg text-sm font-medium transition ${
              mode === "signup"
                ? "bg-white text-gray-900 shadow-sm"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            Sign Up
          </button>
          <button
            type="button"
            onClick={() => setMode("verify")}
            className={`flex-1 py-2 rounded-lg text-sm font-medium transition ${
              mode === "verify"
                ? "bg-white text-gray-900 shadow-sm"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            Verify
          </button>
          <button
            type="button"
            onClick={() => setMode("forgot")}
            className={`flex-1 py-2 rounded-lg text-sm font-medium transition ${
              mode === "forgot" || mode === "reset"
                ? "bg-white text-gray-900 shadow-sm"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            Forgot
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {(mode === "signup" || mode === "forgot") && (
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="Email"
              required
              autoComplete="email"
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
            />
          )}
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Username"
            required
            autoComplete="username"
            className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
          />
          {(mode === "login" || mode === "signup" || mode === "forgot") && (
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={mode === "forgot" ? "Current password (optional)" : "Password"}
              required={mode === "login" || mode === "signup"}
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
            />
          )}
          {mode === "signup" && (
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Confirm password"
              required
              autoComplete="new-password"
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
            />
          )}
          {mode === "verify" && (
            <input
              type="text"
              value={verificationCode}
              onChange={(e) => setVerificationCode(e.target.value)}
              placeholder="Verification code"
              required
              autoComplete="one-time-code"
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
            />
          )}
          {mode === "reset" && (
            <>
              <input
                type="text"
                value={resetCode}
                onChange={(e) => setResetCode(e.target.value)}
                placeholder="Reset code"
                required
                autoComplete="one-time-code"
                className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
              />
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="New password"
                required
                autoComplete="new-password"
                className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 focus:border-transparent text-gray-900 placeholder-gray-400"
              />
            </>
          )}
          <button
            type="submit"
            disabled={loading}
            className="w-full bg-gradient-to-r from-primary-500 to-secondary-500 text-white py-3 rounded-xl font-semibold hover:opacity-90 transition disabled:opacity-60"
          >
            {loading
              ? "Please wait..."
              : mode === "login"
                ? "Login"
                : mode === "signup"
                  ? "Create Account"
                  : mode === "verify"
                    ? "Verify Account"
                    : mode === "forgot"
                      ? "Generate Reset Code"
                      : "Reset Password"}
          </button>
        </form>

        {/* Stats footer */}
        <div className="mt-8 pt-6 border-t border-gray-100">
          <div className="grid grid-cols-3 gap-4 text-center">
            <div>
              <div className="text-xl font-bold text-primary-600">1000+</div>
              <div className="text-xs text-gray-500">Deployments</div>
            </div>
            <div>
              <div className="text-xl font-bold text-primary-600">&lt;10s</div>
              <div className="text-xs text-gray-500">Avg Deploy Time</div>
            </div>
            <div>
              <div className="text-xl font-bold text-primary-600">99.9%</div>
              <div className="text-xs text-gray-500">Uptime</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
