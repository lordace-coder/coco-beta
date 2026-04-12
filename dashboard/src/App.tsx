import { Routes, Route, Navigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { authApi, instanceApi } from "@/api/client";
import { useAuthStore } from "@/hooks/useAuth";
import LoginPage from "@/pages/LoginPage";
import SetupPage from "@/pages/SetupPage";
import Layout from "@/pages/Layout";
import ProjectDetailPage from "@/pages/ProjectDetailPage";
import CollectionDetailPage from "@/pages/CollectionDetailPage";
import SettingsPage from "@/pages/SettingsPage";
import EnvSetupPage from "@/pages/EnvSetupPage";
import SentinelsDocsPage from "@/pages/SentinelsDocsPage";

export default function App() {
  const token = useAuthStore((s) => s.token);

  const { data: setupData, isLoading: setupLoading } = useQuery({
    queryKey: ["setup-status"],
    queryFn: () => authApi.setupStatus().then((r) => r.data),
  });

  // Pre-fetch the instance so useInstance() resolves immediately in child components.
  const { isLoading: instanceLoading } = useQuery({
    queryKey: ["instance"],
    queryFn: () => instanceApi.get().then((r) => r.data),
    enabled: !!token && setupData?.setup_complete === true,
    staleTime: Infinity,
  });

  if (setupLoading || (!!token && setupData?.setup_complete && instanceLoading)) {
    return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">Loading…</div>;
  }

  if (!setupData?.setup_complete) {
    return (
      <Routes>
        <Route path="/setup" element={<SetupPage />} />
        <Route path="*" element={<Navigate to="/setup" replace />} />
      </Routes>
    );
  }

  if (!token) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route path="/setup" element={<Navigate to="/" replace />} />
      <Route path="/login" element={<Navigate to="/" replace />} />
      <Route
        path="/*"
        element={
          <Layout>
            <Routes>
              <Route path="/" element={<ProjectDetailPage />} />
              <Route path="/collections/:colId" element={<CollectionDetailPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/env-setup" element={<EnvSetupPage />} />
              <Route path="/sentinels" element={<SentinelsDocsPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Layout>
        }
      />
    </Routes>
  );
}
