import { Routes, Route, Navigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { authApi } from "@/api/client";
import { useAuthStore } from "@/hooks/useAuth";
import LoginPage from "@/pages/LoginPage";
import SetupPage from "@/pages/SetupPage";
import Layout from "@/pages/Layout";
import OverviewPage from "@/pages/OverviewPage";
import ProjectsPage from "@/pages/ProjectsPage";
import ProjectDetailPage from "@/pages/ProjectDetailPage";
import CollectionDetailPage from "@/pages/CollectionDetailPage";
import SettingsPage from "@/pages/SettingsPage";
import EnvSetupPage from "@/pages/EnvSetupPage";

export default function App() {
  const token = useAuthStore((s) => s.token);

  const { data: setupData, isLoading } = useQuery({
    queryKey: ["setup-status"],
    queryFn: () => authApi.setupStatus().then((r) => r.data),
  });

  if (isLoading) {
    return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">Loading…</div>;
  }

  // First run — no admin exists yet
  if (!setupData?.setup_complete) {
    return (
      <Routes>
        <Route path="/setup" element={<SetupPage />} />
        <Route path="*" element={<Navigate to="/setup" replace />} />
      </Routes>
    );
  }

  // Not logged in
  if (!token) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  // Authenticated
  return (
    <Routes>
      <Route path="/setup" element={<Navigate to="/" replace />} />
      <Route path="/login" element={<Navigate to="/" replace />} />
      <Route
        path="/*"
        element={
          <Layout>
            <Routes>
              <Route path="/" element={<OverviewPage />} />
              <Route path="/projects" element={<ProjectsPage />} />
              <Route path="/projects/:id" element={<ProjectDetailPage />} />
              <Route path="/projects/:id/collections/:colId" element={<CollectionDetailPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/env-setup" element={<EnvSetupPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Layout>
        }
      />
    </Routes>
  );
}
