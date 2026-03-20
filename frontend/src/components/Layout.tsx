import { ReactNode } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { LayoutDashboard, LogOut, Rocket } from "lucide-react";
import toast from "react-hot-toast";
import { useAuthStore } from "../store/authStore";
import { setToken } from "../services/api";

interface LayoutProps {
  children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();

  function handleLogout() {
    logout();
    setToken("");
    toast.success("Logged out");
    navigate("/login");
  }

  const isActive = (path: string) => location.pathname === path;

  return (
    <div className="min-h-screen relative">
      <div className="absolute inset-0 pointer-events-none opacity-60 bg-[radial-gradient(circle_at_10%_10%,rgba(31,123,240,0.12),transparent_42%),radial-gradient(circle_at_90%_5%,rgba(249,115,22,0.12),transparent_35%)]" />

      {/* Mobile top bar */}
      <header className="md:hidden sticky top-0 z-30 border-b border-slate-200/70 bg-white/85 backdrop-blur px-3 py-2.5 sm:px-4 sm:py-3 reveal-up">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Rocket className="text-primary-600" size={20} />
            <h1 className="font-display text-lg font-semibold tracking-tight text-slate-900">InstantDeploy</h1>
          </div>
          <button
            onClick={handleLogout}
            className="focus-ring inline-flex min-h-11 items-center gap-1.5 rounded-lg border border-red-200 px-2.5 py-1.5 text-xs font-medium text-red-700 hover:bg-red-50"
          >
            <LogOut size={14} />
            Logout
          </button>
        </div>
      </header>

      {/* Sidebar */}
      <aside className="hidden md:flex md:w-72 surface-glass border-r border-white/70 flex-col fixed h-full z-20 reveal-up">
        <div className="p-7 border-b border-slate-200/70">
          <div className="flex items-center gap-2.5">
            <Rocket className="text-primary-600" size={24} />
            <h1 className="font-display text-xl font-semibold bg-gradient-to-r from-primary-700 to-secondary-500 bg-clip-text text-transparent">
              InstantDeploy
            </h1>
          </div>
        </div>

        <nav className="flex-1 p-5 space-y-2">
          <Link
            to="/"
            className={`focus-ring flex items-center gap-3 px-4 py-3 rounded-xl transition text-sm font-medium ${
              isActive("/")
                ? "bg-primary-600 text-white shadow-glow"
                : "text-slate-600 hover:bg-white/70 hover:text-slate-900"
            }`}
          >
            <LayoutDashboard size={18} />
            Dashboard
          </Link>
        </nav>

        <div className="p-5 border-t border-slate-200/70">
          <div className="flex items-center gap-3 px-3 py-2 mb-2">
            <div className="w-9 h-9 bg-gradient-to-br from-primary-500 to-secondary-500 rounded-full flex items-center justify-center text-white text-sm font-semibold shrink-0">
              {user?.username?.[0]?.toUpperCase() ?? "U"}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-semibold text-slate-900 truncate">{user?.username ?? "User"}</p>
              <p className="text-xs text-slate-500 capitalize">{user?.role ?? "developer"}</p>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="focus-ring flex min-h-11 items-center gap-2 w-full px-3 py-2 text-sm text-red-700 hover:bg-red-50 rounded-lg transition"
          >
            <LogOut size={16} />
            Logout
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="relative z-10 md:ml-72 min-h-screen px-4 py-5 md:px-8 md:py-8">{children}</main>
    </div>
  );
}
