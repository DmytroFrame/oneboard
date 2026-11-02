import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, PublicOnly, RequireAuth } from "@/context/auth";
import AppLayout from "@/layouts/AppLayout/AppLayout";
import BoardRoute from "@/pages/Board/Board";
import Home from "@/pages/Home/Home";
import Landing from "@/pages/Landing/Landing";
import Login from "@/pages/Login/Login";
import OrganizationPage from "@/pages/Organization/Organization";
import Register from "@/pages/Register/Register";
import SettingsPage from "@/pages/Settings/Settings";

export function App() {
  return (
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
  );
}
