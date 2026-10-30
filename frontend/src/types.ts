export type Priority = "low" | "medium" | "high" | "critical";

export interface User {
  id: number;
  name: string;
  email: string;
  color: string;
  avatarUrl: string | null;
}

export type Locale = "en" | "uk" | "de";
export type LanguagePref = "auto" | Locale;
export type ThemePref = "auto" | "light" | "dark";

export interface Settings {
  language: LanguagePref;
  theme: ThemePref;
}

/** The signed-in user: settings are only returned for yourself. */
export interface CurrentUser extends User {
  settings: Settings;
}

export type Role = "owner" | "member";

export interface Member extends User {
  role: Role;
  assignedCount: number;
}

export interface Column {
  id: number;
  projectId: number;
  name: string;
  position: number;
  cardCount: number;
}

export type OrgRole = "owner" | "admin" | "member";

export interface Organization {
  id: number;
  name: string;
  role: OrgRole; // the current user's role
  memberCount: number;
  projectCount: number;
  createdAt: string;
}

export interface OrgMember extends User {
  role: OrgRole;
  addedAt: string;
}

export interface Invitation {
  id: number;
  email: string;
  role: OrgRole;
  createdAt: string;
}

export interface Project {
  id: number;
  organizationId: number;
  organizationName: string;
  name: string;
  key: string;
  ownerId: number;
  role: Role;
  memberCount: number;
  cardCount: number;
  createdAt: string;
}

export interface Card {
  id: number;
  projectId: number;
  columnId: number;
  key: string;
  title: string;
  description: string;
  priority: Priority;
  position: number;
  authorId: number;
  assigneeId: number | null;
  createdAt: string;
  updatedAt: string;
  attachmentCount: number;
  cover: CardCover | null; // first image or video, shown on the card face
}

export type AttachmentKind = "image" | "video" | "file";

export interface CardCover {
  kind: AttachmentKind;
  url: string;
}

export interface Attachment {
  id: number;
  cardId: number;
  fileName: string;
  contentType: string;
  size: number;
  kind: AttachmentKind;
  uploaderId: number | null;
  createdAt: string;
  url: string;
  downloadUrl: string;
  thumbUrl: string | null;
}

export interface AssignedCard extends Card {
  projectName: string;
  columnName: string;
}

export interface Dashboard {
  stats: { organizations: number; projects: number; assigned: number; urgent: number; created: number };
  assigned: AssignedCard[];
}

export interface CardInput {
  title: string;
  description: string;
  priority: Priority;
  assigneeId: number | null;
  columnId?: number;
}

export interface CardPage {
  cards: Card[];
  nextCursor: string | null;
  total: number;
}

export interface CardFilters {
  q: string;
  assignees: number[];
  unassigned: boolean;
}

export const PRIORITIES: Priority[] = ["low", "medium", "high", "critical"];
