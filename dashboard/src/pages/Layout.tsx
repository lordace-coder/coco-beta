import { Link, useLocation } from "react-router-dom";
import { useAuthStore } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";

const VERSION = "0.1.0";

const navItems = [
  { label: "Overview", href: "/" },
  { label: "Projects", href: "/projects" },
  { label: "Settings", href: "/settings" },
  { label: "Environment", href: "/env-setup" },
  { label: "Sentinels", href: "/sentinels" },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout, admin } = useAuthStore();

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="flex w-56 flex-col border-r bg-card">
        <div className="border-b px-4 py-4">
          <span className="text-lg font-bold tracking-tight">Cocobase</span>
          <p className="text-xs text-muted-foreground">Admin Dashboard · v{VERSION}</p>
        </div>
        <nav className="flex-1 space-y-1 p-3">
          {navItems.map((item) => (
            <Link
              key={item.href}
              to={item.href}
              className={cn(
                "block rounded-md px-3 py-2 text-sm font-medium transition-colors",
                location.pathname === item.href
                  ? "bg-primary text-primary-foreground"
                  : "text-foreground hover:bg-accent"
              )}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="border-t p-3">
          <p className="truncate text-xs text-muted-foreground">{admin?.email}</p>
          <button
            onClick={logout}
            className="mt-1 text-xs text-destructive hover:underline"
          >
            Sign out
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-background">
        <div className="mx-auto max-w-6xl p-6">{children}</div>
      </main>
    </div>
  );
}
