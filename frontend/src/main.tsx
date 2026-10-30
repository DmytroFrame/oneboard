import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, PublicOnly, RequireAuth } from "./auth";
import { initI18n } from "./i18n";
import { storedSettings } from "./preferences";
import Landing from "./pages/Landing";
import { Login, Register } from "./pages/AuthPages";
import BoardRoute from "./pages/Board";
import AppLayout from "./pages/AppLayout";
import Home from "./pages/Home";
import OrganizationPage from "./pages/Organization";
import SettingsPage from "./pages/Settings";
import "./styles.css";

// Until the session is restored, use the language this browser used last time.
initI18n(storedSettings().language);

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          {/* Signed-in users never see the marketing pages: they land on /home. */}
          <Route element={<PublicOnly />}>
            <Route path="/" element={<Landing />} />
            <Route path="/login" element={<Login />} />
            <Route path="/register" element={<Register />} />
          </Route>
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route path="/home" element={<Home />} />
            <Route path="/orgs/:orgId" element={<OrganizationPage />} />
            <Route path="/projects/:projectId" element={<BoardRoute />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
          <Route path="/projects" element={<Navigate to="/home" replace />} />
          <Route path="/board" element={<Navigate to="/home" replace />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
