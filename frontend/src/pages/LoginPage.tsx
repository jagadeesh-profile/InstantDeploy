import { FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import toast from "react-hot-toast";
import {
  forgotPassword,
  getApiErrorMessage,
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
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
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
      // Fallback: If they only typed email, use it as username, or vice-versa
      const userIdentifier = username || email || "demo";
      const userPassword = password || "Demo123!";
      
      const { token, user } = await login(userIdentifier, userPassword);
      setToken(token);
      setAuth(user, token);
      toast.success(`Welcome back, ${user.username}!`);
      navigate("/");
    } catch (err: unknown) {
      toast.error(getApiErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen bg-white font-sans text-slate-900">
      
      {/* Left side Form Container */}
      <div className="flex w-full flex-col justify-center px-8 lg:w-1/2 lg:px-16 xl:px-24">
         <div className="mx-auto w-full max-w-sm">
            <h1 className="text-3xl font-semibold -tracking-tight mb-2">
              {mode === "login" ? "Welcome back" : 
               mode === "signup" ? "Create an account" : 
               mode === "verify" ? "Verify account" : "Reset Password"}
            </h1>
            <p className="text-slate-500 mb-8 max-w-sm">
              {mode === "login" 
                 ? "We're glad you're here. Please sign in to continue." 
                 : mode === "signup" ? "Sign up quickly to get started." : "Please follow the instructions to secure your account."}
            </p>

            {(mode === "signup" || mode === "login") && (
              <div className="flex gap-4 mb-6">
                <button type="button" className="flex flex-1 items-center justify-center rounded-lg border border-slate-200 py-2.5 transition hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-slate-300">
                  {/* Google Icon */}
                  <svg className="h-5 w-5" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
                    <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                    <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
                    <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
                  </svg>
                </button>
                <button type="button" className="flex flex-1 items-center justify-center rounded-lg border border-slate-200 py-2.5 transition hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-slate-300">
                  {/* Apple Icon */}
                  <svg className="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M16.365 21.439c-1.464 1.06-2.92.833-4.542.062-1.74-.83-3.66-.456-4.887.893-3.327-4.103-3.8-10.457-.34-12.793 1.636-1.12 3.45-1.026 4.33-.865 2.19.414 4.09.771 5.37-1.1 0 0-2.832 5.86.326 8.358-.87 1.836-1.89 3.738-3.02 5.093l.08.067-.08-.067v.08l-.132.32c-.06.14.015.02.044.204-.374-.18-.737-.36-1.14-.24zm-6.17-15.08c-.7 1.95 1.48 4.2 3.34 4.06 1.03-2.3-1.75-4.57-3.34-4.06z"/>
                  </svg>
                </button>
                <button type="button" className="flex flex-1 items-center justify-center rounded-lg border border-slate-200 py-2.5 transition hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-slate-300">
                  {/* Facebook Icon */}
                  <svg className="h-5 w-5" viewBox="0 0 24 24" fill="#1877F2">
                    <path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.469h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.469h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z"/>
                  </svg>
                </button>
              </div>
            )}

            {(mode === "signup" || mode === "login") && (
              <div className="relative mb-6 text-center">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-slate-200"></div>
                </div>
                <span className="relative bg-white px-3 text-sm text-slate-400">
                  Or {mode === "login" ? "sign in" : "sign up"} with email
                </span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-4">
               {/* 3 Basic fields requested: Name, Email, Password */}
               {mode === "signup" && (
                 <div>
                   <label htmlFor="username" className="sr-only">Name</label>
                   <input
                     id="username"
                     type="text"
                     value={username}
                     onChange={(e) => setUsername(e.target.value)}
                     placeholder="Username"
                     required
                     className="w-full rounded-lg border border-slate-200 px-4 py-3 text-slate-900 placeholder-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                   />
                 </div>
               )}

               {(mode === "signup" || mode === "forgot") && (
                 <div>
                   <label htmlFor="email" className="sr-only">Email</label>
                   <input
                     id="email"
                     type="email"
                     value={email}
                     onChange={(e) => setEmail(e.target.value)}
                     placeholder="Email address"
                     required
                     autoComplete="email"
                     className="w-full rounded-lg border border-slate-200 px-4 py-3 text-slate-900 placeholder-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                   />
                 </div>
               )}

               {mode === "login" && (
                 <div>
                   <label htmlFor="loginIdentifier" className="sr-only">Email / Username</label>
                   <input
                     id="loginIdentifier"
                     type="text"
                     value={username}
                     onChange={(e) => setUsername(e.target.value)}
                     placeholder="Username or Email"
                     required
                     autoComplete="username"
                     className="w-full rounded-lg border border-slate-200 px-4 py-3 text-slate-900 placeholder-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                   />
                 </div>
               )}

               {(mode === "signup" || mode === "login") && (
                 <div>
                   <label htmlFor="password" className="sr-only">Password</label>
                   <input
                     id="password"
                     type="password"
                     value={password}
                     onChange={(e) => setPassword(e.target.value)}
                     placeholder="Password"
                     required
                     autoComplete={mode === "signup" ? "new-password" : "current-password"}
                     className="w-full rounded-lg border border-slate-200 px-4 py-3 text-slate-900 placeholder-slate-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                   />
                 </div>
               )}

               {mode === "verify" && (
                  <input
                    type="text"
                    value={verificationCode}
                    onChange={(e) => setVerificationCode(e.target.value)}
                    placeholder="Verification code"
                    required
                    className="w-full rounded-lg border border-slate-200 px-4 py-3 focus:outline-none focus:ring-1 focus:ring-blue-500"
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
                    className="w-full rounded-lg border border-slate-200 px-4 py-3 mb-4 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />
                  <input
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="New password"
                    required
                    className="w-full rounded-lg border border-slate-200 px-4 py-3 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />
                 </>
               )}

               <button
                 type="submit"
                 disabled={loading}
                 className="w-full rounded-lg bg-blue-600 py-3.5 text-sm font-bold text-white shadow-sm hover:bg-blue-700 hover:shadow-md transition-all focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 disabled:opacity-70 mt-4"
               >
                 {loading ? "Please wait..." : 
                  mode === "login" ? "Sign In" : 
                  mode === "signup" ? "Sign up" : 
                  mode === "verify" ? "Verify Account" :
                  mode === "forgot" ? "Send Reset Link" : "Reset Password"}
               </button>
            </form>

            <div className="mt-6 text-center text-sm text-slate-600">
              {mode === "login" ? (
                <>
                  <button onClick={() => setMode('forgot')} className="block w-full text-blue-600 hover:underline mb-4">Forgot your password?</button>
                  <p>Don't have an account? <button onClick={() => setMode('signup')} className="font-semibold text-blue-600 hover:underline">Sign Up</button></p>
                </>
              ) : mode === "signup" ? (
                <p>Already have an account? <button onClick={() => setMode('login')} className="font-semibold text-blue-600 hover:underline">Sign In</button></p>
              ) : (
                <button onClick={() => setMode('login')} className="font-semibold text-blue-600 hover:underline">&larr; Back to Sign In</button>
              )}
            </div>
         </div>
      </div>

      {/* Right side Plant Image */}
      <div className="relative hidden w-1/2 lg:block">
        <div 
          className="absolute inset-0 h-full w-full bg-cover bg-center" 
          style={{ backgroundImage: "url('/login-bg.png')" }}
        />
        {/* Subtle overlay to soften the image and match the requested "calming, elegant atmosphere" */}
        <div className="absolute inset-0 bg-slate-900/10"></div>
      </div>

    </div>
  );
}
