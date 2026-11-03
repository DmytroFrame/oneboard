import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { CallBar } from "@/components/CallBar/CallBar";
import { CreateOrgModal } from "@/components/CreateOrgModal/CreateOrgModal";
import { CreateProjectModal } from "@/components/CreateProjectModal/CreateProjectModal";
import { Sidebar } from "@/components/Sidebar/Sidebar";
import { Toast } from "@/components/Toast/Toast";
import { Topbar } from "@/components/Topbar/Topbar";
import { Banner } from "@/components/ui/Banner/Banner";
import { Center } from "@/components/ui/Center/Center";
import { useAuth } from "@/context/auth";
import { useEscape } from "@/hooks/useEscape";
import { useVoiceProblemText } from "@/hooks/useVoiceProblemText";
import { api } from "@/lib/api";
import { TIMELOGS_CHANGED, timeApi } from "@/lib/api/time";
import { canAccess } from "@/lib/companyRoles";
import { can } from "@/lib/companyRoles";
import { BoardSocket, type UserEvent } from "@/lib/realtime";
import { useVoice, voice } from "@/lib/voice";
import type { Organization, Project } from "@/types";
import type { LayoutContext } from "./useLayout";
import s from "./AppLayout.module.scss";

/** The signed-in app shell: top bar, sidebar, notices, and the user's real-time channel. */
export default function AppLayout() {
  const { t } = useTranslation();
  const { user, setUser } = useAuth();
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [error, setError] = useState("");
  const [creatingProject, setCreatingProject] = useState<{ orgId?: number } | null>(null);
  const [creatingOrg, setCreatingOrg] = useState(false);
  const [notice, setNotice] = useState<{ text: string; tone: "info" | "warn" } | null>(null);
  const [orgMembersSignal, setOrgMembersSignal] = useState({ orgId: 0, n: 0 });
  const location = useLocation();
  const pathRef = useRef(location.pathname);
  pathRef.current = location.pathname;
  const projectsRef = useRef(projects);
  projectsRef.current = projects;
  // Phones: the sidebar is a drawer, closed on every navigation.
  const [drawerOpen, setDrawerOpen] = useState(false);
  useEffect(() => setDrawerOpen(false), [location.pathname]);
  const closeDrawer = useCallback(() => setDrawerOpen(false), []);

  // Sidebar dots: missing time logs, absence requests to review.
  const [alerts, setAlerts] = useState({ time: false, vacations: false });
  const loadAlerts = useCallback(() => {
    if (canAccess(user, "time")) timeApi.missing().then((m) => setAlerts((a) => ({ ...a, time: m.days.length > 0 }))).catch(() => {});
    if (canAccess(user, "vacations")) timeApi.toReview().then((v) => setAlerts((a) => ({ ...a, vacations: v.length > 0 }))).catch(() => {});
  }, [user]);
  useEffect(() => {
    loadAlerts();
    window.addEventListener(TIMELOGS_CHANGED, loadAlerts);
    return () => window.removeEventListener(TIMELOGS_CHANGED, loadAlerts);
  }, [loadAlerts]);
  useEscape(closeDrawer, drawerOpen);

  const reload = useCallback(async () => {
    try {
      const [o, p] = await Promise.all([api.organizations(), api.projects()]);
      setOrgs(o);
      setProjects(p);
    } catch (err) {
      setError((err as Error).message);
      setProjects((p) => p ?? []);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => {
    const on = () => {
      setOnline(true);
      void reload(); // whatever failed while offline
    };
    const off = () => setOnline(false);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, [reload]);

  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(null), 6000);
    return () => clearTimeout(timer);
  }, [notice]);

  // The user's own real-time channel: access granted or taken away anywhere in the app.
  useEffect(() => {
    const socket = new BoardSocket(0);
    voice.attach(socket); // voice rooms are signaled over this channel, so calls survive navigation
    const off = socket.on("user", (m) => {
      const e = m.event as UserEvent;
      if (e.type === "voice") return; // handled by the voice controller
      if (e.type === "vacations") {
        loadAlerts();
        return;
      }
      if (e.type === "account") {
        // Our company role changed: reload the account (permissions, sections).
        api.me().then(setUser).catch(() => {});
        return;
      }
      if (e.type === "org.members") {
        setOrgMembersSignal((prev) => ({ orgId: e.orgId, n: prev.n + 1 }));
        void reload(); // member counts
        return;
      }
      void reload();
      const self = e.actorId === user!.id;
      const name = e.name;
      if (e.reason === "added" && !self) setNotice({ text: t("live.addedTo", { name }), tone: "info" });
      if (e.reason === "role" && !self) setNotice({ text: t("live.roleChanged", { name }), tone: "info" });
      if (e.reason === "removed" || e.reason === "deleted") {
        // Leave a page we can no longer see.
        const path = pathRef.current;
        const onIt = e.projectId
          ? path === `/projects/${e.projectId}`
          : path === `/orgs/${e.orgId}` || !!projectsRef.current?.some((p) => p.organizationId === e.orgId && path === `/projects/${p.id}`);
        if (!self) setNotice({ text: t(e.reason === "deleted" ? "live.deleted" : "live.removedFrom", { name }), tone: "warn" });
        if (onIt) navigate("/home", { replace: true });
      }
    });
    return () => {
      off();
      voice.detach();
      socket.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reload]);

  // Voice: people joining or leaving the room you are in, or of the board you are looking at.
  const onBoard = (projectId: number) => location.pathname === `/projects/${projectId}`;
  useEffect(
    () =>
      voice.onRoomChange((projectId, joined, left) => {
        if (voice.get().call?.projectId !== projectId && pathRef.current !== `/projects/${projectId}`) return;
        const name = (joined[0] ?? left[0])!.name;
        setNotice({ text: t(joined.length ? "voice.joinedToast" : "voice.leftToast", { name }), tone: "info" });
      }),
    [t],
  );
  // Voice problems are shown in the board's dock; elsewhere (e.g. access revoked mid-call) as a toast.
  const { problem } = useVoice();
  const problemText = useVoiceProblemText();
  useEffect(() => {
    if (!problem || pathRef.current === `/projects/${problem.projectId}`) return;
    setNotice({ text: problemText(problem.reason), tone: "warn" });
    voice.clearProblem();
  }, [problem]); // eslint-disable-line react-hooks/exhaustive-deps

  const ctx: LayoutContext = {
    projects: projects ?? [],
    orgs,
    reload,
    openCreateProject: (orgId) => setCreatingProject({ orgId }),
    openCreateOrg: () => setCreatingOrg(true),
    orgMembersSignal,
  };

  return (
    <div className={s.app}>
      <Topbar drawerOpen={drawerOpen} onToggleDrawer={() => setDrawerOpen((v) => !v)} />

      {!online && <div className={s.offline}>{t("pwa.offline")}</div>}
      <CallBar projectName={(id) => projects?.find((p) => p.id === id)?.name} onBoard={onBoard} />

      <div className={s.body}>
        {drawerOpen && <div className={s.drawerBackdrop} onClick={closeDrawer} />}
        <Sidebar
          user={user!}
          orgs={orgs}
          projects={projects ?? []}
          open={drawerOpen}
          onCreateProject={(orgId) => setCreatingProject({ orgId })}
          onCreateOrg={() => setCreatingOrg(true)}
          alerts={alerts}
        />

        {notice && (
          <Toast tone={notice.tone} onClick={() => setNotice(null)}>
            {notice.text}
          </Toast>
        )}
        <main className={s.main}>
          {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}
          {projects === null ? <Center>{t("common.loading")}</Center> : <Outlet context={ctx} />}
        </main>
      </div>

      {creatingProject && (
        <CreateProjectModal
          orgs={orgs.filter((o) => o.id === user?.organization?.id && can(user, "addProject"))}
          defaultOrgId={creatingProject.orgId}
          onClose={() => setCreatingProject(null)}
          onCreated={async (p) => {
            await reload();
            navigate(`/projects/${p.id}`);
          }}
        />
      )}
      {creatingOrg && (
        <CreateOrgModal
          onClose={() => setCreatingOrg(false)}
          onCreated={async (o) => {
            await reload();
            navigate(`/orgs/${o.id}`);
          }}
        />
      )}
    </div>
  );
}
