import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, RequireAuth } from "./auth";
import { initI18n } from "./i18n";
import { storedSettings } from "./preferences";
import Landing from "./pages/Landing";
import { Login, Register } from "./pages/AuthPages";
import BoardRoute from "./pages/Board";
import AppLayout, { ProjectsHome } from "./pages/AppLayout";
import SettingsPage from "./pages/Settings";
import "./styles.css";

// Until /auth/me answers, use the language this browser used last time.
initI18n(storedSettings().language);

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route path="/projects" element={<ProjectsHome />} />
            <Route path="/projects/:projectId" element={<BoardRoute />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
          <Route path="/board" element={<Navigate to="/projects" replace />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
