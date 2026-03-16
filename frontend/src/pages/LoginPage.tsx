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

type Mode = "login" | "signup" | "verify" | "forgot" | "reset";

export default function LoginPage() {
  const navigate = useNavigate();
  const { setAuth, isAuthenticated } = useAuthStore();

  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("demo");
  const [password, setPassword] = useState("Demo123!");
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
          return;
        }
        const res = await signup(email, username, password);
        if (res.verificationCode) {
          toast.success(`Account created! Verification code: ${res.verificationCode}`);
          setVerificationCode(res.verificationCode);
        } else {
          toast.success("Account created — check your email for the verification code.");
        }
        setMode("verify");
        return;
      }

      if (mode === "verify") {
        await verifyAccount(username, verificationCode);
        toast.success("Account verified — you can now log in.");
        setMode("login");
        return;
      }

      if (mode === "forgot") {
        const res = await forgotPassword(username, email);
        if (res.resetCode) {
          setResetCode(res.resetCode);
          toast.success(`Reset code: ${res.resetCode}`);
        } else {
          toast.success("Reset code generated.");
        }
        setMode("reset");
        return;
      }

      if (mode === "reset") {
        await resetPassword(username, resetCode, newPassword);
        toast.success("Password reset — please log in.");
        setMode("login");
        return;
      }

      // Login
      const { token, user } = await login(username, password);
      setToken(token);
      setAuth(user, token);
      toast.success(`Welcome back, ${user.username}!`);
      navigate("/");
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } };
      const msg = axiosErr?.response?.data?.error ?? "Request failed";
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  const modeLabel: Record<Mode, string> = {
    login:  "Login",
    signup: "Sign Up",
    verify: "Verify",
    forgot: "Forgot",
    reset:  "Reset",
  };

  const submitLabel: Record<Mode, string> = {
    login:  "Login",
    signup: "Create Account",
    verify: "Verify Account",
    forgot: "Generate Reset Code",
    reset:  "Reset Password",
  };

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
            Deploy repositories to Docker containers in seconds.
          </p>
          {mode === "login" && (
            <p className="text-xs text-gray-400 mt-2">
              Demo: <code className="bg-gray-100 px-1 rounded">demo</code> /
              <code className="bg-gray-100 px-1 rounded ml-1">Demo123!</code>
            </p>
          )}
        </div>

        {/* Mode tabs */}
        <div className="flex bg-gray-100 rounded-xl p-1 mb-6 gap-1">
          {(["login", "signup", "verify", "forgot"] as Mode[]).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`flex-1 py-2 rounded-lg text-xs font-medium transition ${
                mode === m || (mode === "reset" && m === "forgot")
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {modeLabel[m]}
            </button>
          ))}
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Email field */}
          {(mode === "signup" || mode === "forgot") && (
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="Email"
              required
              autoComplete="email"
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
            />
          )}

          {/* Username */}
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Username"
            required
            autoComplete="username"
            className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
          />

          {/* Password */}
          {(mode === "login" || mode === "signup") && (
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Password"
              required
              autoComplete={mode === "signup" ? "new-password" : "current-password"}
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
            />
          )}

          {/* Confirm password */}
          {mode === "signup" && (
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Confirm password"
              required
              autoComplete="new-password"
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
            />
          )}

          {/* Verification code */}
          {mode === "verify" && (
            <input
              type="text"
              value={verificationCode}
              onChange={(e) => setVerificationCode(e.target.value)}
              placeholder="Verification code"
              required
              autoComplete="one-time-code"
              className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
            />
          )}

          {/* Reset fields */}
          {mode === "reset" && (
            <>
              <input
                type="text"
                value={resetCode}
                onChange={(e) => setResetCode(e.target.value)}
                placeholder="Reset code"
                required
                className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
              />
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="New password (e.g. MyPass1!)"
                required
                autoComplete="new-password"
                className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-400 text-gray-900 placeholder-gray-400"
              />
            </>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-gradient-to-r from-primary-500 to-secondary-500 text-white py-3 rounded-xl font-semibold hover:opacity-90 transition disabled:opacity-60"
          >
            {loading ? "Please wait…" : submitLabel[mode]}
          </button>
        </form>

        {mode === "signup" && (
          <p className="text-xs text-gray-400 mt-4 text-center">
            Password must be 8+ chars with uppercase, lowercase, number, and special character.
          </p>
        )}

        <div className="mt-8 pt-6 border-t border-gray-100">
          <div className="grid grid-cols-3 gap-4 text-center">
            {[
              { label: "Deployments", value: "1000+" },
              { label: "Avg Deploy",  value: "<2min" },
              { label: "Uptime",      value: "99.9%" },
            ].map(({ label, value }) => (
              <div key={label}>
                <div className="text-xl font-bold text-primary-600">{value}</div>
                <div className="text-xs text-gray-500">{label}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
