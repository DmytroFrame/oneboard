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

export type CompanyRole =
  | "OWNER"
  | "ADMIN"
  | "MANAGER"
  | "RECRUITER"
  | "HR"
  | "SALES"
  | "EMPLOYEE"
  | "CONTRACTOR"
  | "ACCOUNTANT"
  | "CANDIDATE";
export type AdditionRole = "LEAD";

/** A company permission (backend/internal/domain/company.go is the only copy of the matrix). */
export type Permission =
  | "addUser" | "editUser" | "fireUser" | "editGrades" | "viewSalary" | "viewCandidates" | "addDocs"
  | "resetPassword" | "fireYourself" | "editProjectComment" | "addTask" | "openBackwoods"
  | "editProfile" | "editCandidate" | "viewGrades" | "viewMoreTeamInfo" | "viewUserComments"
  | "createWikiArticle" | "viewCrewBlock" | "addOwnDocs" | "deleteDocs" | "editDocs" | "seeHRInformation"
  | "changeAdditionRole" | "viewPrivateDocs" | "seeProjectMembers" | "addProjectMembers" | "addProject"
  | "editProject" | "editAllStatuses" | "addProjectCharters" | "editWorklogs" | "seeVacationsRequests"
  | "viewAllProjects" | "editBudget" | "editHolidays" | "viewTrialUser" | "accessStatistics"
  | "seeFilterInVacationStatistics" | "deleteItemsFromStatistics" | "viewGlobalSearch" | "showBirthdays"
  | "editUserFields";

/** One of the user's organizations (each is an isolated workspace with its own role). */
export interface Membership {
  id: number;
  name: string;
  role: CompanyRole;
  active: boolean;
}

/** The signed-in user in the organization they work in: settings, role and permissions are per user. */
export interface CurrentUser extends User {
  settings: Settings;
  /** null: not in any organization yet (they can create one). */
  organization: Membership | null;
  organizations: Membership[];
  /** Runs the installation (backups, schema); not an organization role. */
  platformAdmin: boolean;
  /** Company role in the current organization ("" outside of one). */
  role: CompanyRole | "";
  additionRoles: AdditionRole[];
  /** What the role allows, computed by the server. */
  permissions: Permission[];
  /** Roles this user may give to others (empty: can't change roles). */
  assignableRoles: CompanyRole[];
}

/** A project role; "" = not a member (seen through the viewAllProjects permission). */
export type Role = "owner" | "admin" | "member" | "";

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
  status: ColumnStatus;
}

export type ColumnStatus = "backlog" | "todo" | "in_progress" | "testing" | "done" | "rejected";
export const COLUMN_STATUSES: ColumnStatus[] = ["backlog", "todo", "in_progress", "testing", "done", "rejected"];

export interface Organization {
  id: number;
  name: string;
  role: CompanyRole; // the current user's role there
  memberCount: number;
  projectCount: number;
  createdAt: string;
}

export interface OrgMember extends User {
  role: CompanyRole;
  additionRoles: AdditionRole[];
  active: boolean;
  addedAt: string;
}

export interface Invitation {
  id: number;
  email: string;
  role: CompanyRole;
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
  pinned: boolean;
  client: string;
  description: string;
  startDate: string | null;
  active: boolean;
  billable: boolean;
  budgetHours: number | null;
  isMain: boolean;
  sections: ProjectSections;
}

export interface ProjectSections {
  backlog: boolean;
  planning: boolean;
  comments: boolean;
  statistics: boolean;
  wiki: boolean;
}

export type ProjectSettings = Pick<Project, "client" | "description" | "startDate" | "active" | "billable" | "budgetHours" | "isMain" | "sections">;

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
  // OmiTeam task fields
  storyPoints: number | null;
  budgetHours: number | null;
  billable: boolean;
  specialType: SpecialType;
  milestoneId: number | null;
  sprintId: number | null;
  versionId: number | null;
  isTemplate: boolean;
  tags: Tag[];
  checklistDone: number;
  checklistTotal: number;
  commentCount: number;
  trackedMinutes: number;
}

export type SpecialType = "" | "sick_day" | "day_off" | "vacation" | "pre_sale" | "bench" | "interview";
export const SPECIAL_TYPES: SpecialType[] = ["", "sick_day", "day_off", "vacation", "pre_sale", "bench", "interview"];

export interface Tag {
  id: number;
  name: string;
  color: string;
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

/** Someone's yearly date that is coming up (birthday, work anniversary) or a trial end. */
export interface PersonDate {
  user: User;
  date: string;
  years?: number;
}

/** Company-wide dashboard widgets; null lists are hidden for the viewer. */
export interface CompanyWidgets {
  counters: { employees: number; contractors: number; myProjects: number; candidates: number | null };
  birthdays: PersonDate[] | null;
  anniversaries: PersonDate[];
  onTrial: PersonDate[] | null;
}

export interface DayWork {
  day: string;
  billableMinutes: number;
  otherMinutes: number;
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
  milestone?: number | null;
  sprint?: number | null;
  version?: number | null;
  tags?: number[];
}

export const PRIORITIES: Priority[] = ["low", "medium", "high", "critical"];
