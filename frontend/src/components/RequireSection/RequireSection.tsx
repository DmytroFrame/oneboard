import type { ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { StatusPage } from "@/components/StatusPage/StatusPage";
import { useAuth } from "@/context/auth";
import { canAccess, homeRoute, type Section } from "@/lib/companyRoles";

interface Props {
  section: Section;
  /** For the default landing page: send people who can't see it to their own home instead of a 403. */
  redirect?: boolean;
  children: ReactNode;
}

/** Renders the page only if the user's company role includes its section. */
export function RequireSection({ section, redirect, children }: Props) {
  const { user } = useAuth();
  if (canAccess(user, section)) return <>{children}</>;
  if (redirect) return <Navigate to={homeRoute(user)} replace />;
  return <StatusPage code={403} />;
}
