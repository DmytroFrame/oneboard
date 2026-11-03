import { lazy, Suspense, type ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { RequireSection } from "@/components/RequireSection/RequireSection";
import { StatusPage } from "@/components/StatusPage/StatusPage";
import { AuthProvider, PublicOnly, RequireAuth } from "@/context/auth";
import AppLayout from "@/layouts/AppLayout/AppLayout";
import BoardRoute from "@/pages/Board/Board";
import Home from "@/pages/Home/Home";
import Landing from "@/pages/Landing/Landing";
import Login from "@/pages/Login/Login";
import OrganizationPage from "@/pages/Organization/Organization";
import Register from "@/pages/Register/Register";
import ForgotPassword from "@/pages/ForgotPassword/ForgotPassword";
import ResetPassword from "@/pages/ResetPassword/ResetPassword";
import SettingsPage from "@/pages/Settings/Settings";
import Profile from "@/pages/Profile/Profile";
import Team from "@/pages/Team/Team";
import Projects from "@/pages/Projects/Projects";
import Backlog from "@/pages/Backlog/Backlog";
import Backwoods from "@/pages/Backwoods/Backwoods";
import Candidates from "@/pages/Candidates/Candidates";
import HRStatistics from "@/pages/HRStatistics/HRStatistics";
import Planning from "@/pages/Planning/Planning";
import ProjectStatistics from "@/pages/ProjectStatistics/ProjectStatistics";
import Reports from "@/pages/Reports/Reports";
import TimeTracking from "@/pages/TimeTracking/TimeTracking";
import Vacations from "@/pages/Vacations/Vacations";
import ProjectComments from "@/pages/ProjectComments/ProjectComments";
import ProjectWiki from "@/pages/ProjectWiki/ProjectWiki";
import Wiki from "@/pages/Wiki/Wiki";
import WikiGroups from "@/pages/WikiGroups/WikiGroups";
import WikiHistory from "@/pages/WikiHistory/WikiHistory";

// Pages with the TipTap editor are loaded on demand, like the card modal.
const WikiArticle = lazy(() => import("@/pages/WikiArticle/WikiArticle"));
const WikiEditor = lazy(() => import("@/pages/WikiEditor/WikiEditor"));

function WikiSection({ children }: { children: ReactNode }) {
  return (
    <RequireSection section="wiki">
      <Suspense fallback={null}>{children}</Suspense>
    </RequireSection>
  );
}

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
            <Route path="/forgot-password" element={<ForgotPassword />} />
          </Route>
          {/* Works signed in or out: the link comes from an email. */}
          <Route path="/reset-password/:token" element={<ResetPassword />} />
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route
              path="/home"
              element={
                <RequireSection section="home" redirect>
                  <Home />
                </RequireSection>
              }
            />
            <Route
              path="/orgs/:orgId"
              element={
                <RequireSection section="projects">
                  <OrganizationPage />
                </RequireSection>
              }
            />
            <Route
              path="/projects/:projectId"
              element={
                <RequireSection section="projects">
                  <BoardRoute />
                </RequireSection>
              }
            />
            <Route
              path="/team"
              element={
                <RequireSection section="team">
                  <Team />
                </RequireSection>
              }
            />
            <Route
              path="/projects"
              element={
                <RequireSection section="projects">
                  <Projects />
                </RequireSection>
              }
            />
            <Route path="/people/:id" element={<Profile />} />
            <Route path="/profile" element={<Profile />} />
            <Route
              path="/projects/:projectId/backlog"
              element={
                <RequireSection section="projects">
                  <Backlog />
                </RequireSection>
              }
            />
            <Route
              path="/projects/:projectId/planning"
              element={
                <RequireSection section="projects">
                  <Planning />
                </RequireSection>
              }
            />
            <Route
              path="/projects/:projectId/statistics"
              element={
                <RequireSection section="projects">
                  <ProjectStatistics />
                </RequireSection>
              }
            />
            <Route
              path="/reports/:kind?"
              element={
                <RequireSection section="reports">
                  <Reports />
                </RequireSection>
              }
            />
            <Route path="/wiki" element={<WikiSection><Wiki /></WikiSection>} />
            <Route path="/wiki/groups" element={<WikiSection><WikiGroups /></WikiSection>} />
            <Route path="/wiki/new" element={<WikiSection><WikiEditor /></WikiSection>} />
            <Route path="/wiki/:id" element={<WikiSection><WikiArticle /></WikiSection>} />
            <Route path="/wiki/:id/edit" element={<WikiSection><WikiEditor /></WikiSection>} />
            <Route path="/wiki/:id/history" element={<WikiSection><WikiHistory /></WikiSection>} />
            <Route
              path="/projects/:projectId/wiki"
              element={
                <RequireSection section="projects">
                  <ProjectWiki />
                </RequireSection>
              }
            />
            <Route
              path="/projects/:projectId/comments"
              element={
                <RequireSection section="projects">
                  <ProjectComments />
                </RequireSection>
              }
            />
            <Route
              path="/backwoods"
              element={
                <RequireSection section="backwoods">
                  <Backwoods />
                </RequireSection>
              }
            />
            <Route
              path="/candidates"
              element={
                <RequireSection section="candidates">
                  <Candidates />
                </RequireSection>
              }
            />
            <Route
              path="/statistics"
              element={
                <RequireSection section="hrStatistics">
                  <HRStatistics />
                </RequireSection>
              }
            />
            <Route
              path="/time"
              element={
                <RequireSection section="time">
                  <TimeTracking />
                </RequireSection>
              }
            />
            <Route
              path="/vacations"
              element={
                <RequireSection section="vacations">
                  <Vacations />
                </RequireSection>
              }
            />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<StatusPage code={404} />} />
          </Route>
          <Route path="/board" element={<Navigate to="/home" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
