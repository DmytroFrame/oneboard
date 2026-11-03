import type { AdditionRole, CompanyRole, User } from "./index";

export interface EmergencyContact {
  name?: string;
  relation?: string;
  phone?: string;
  telegram?: string;
}

export interface ProfileDetails {
  about?: string;
  phone?: string;
  telegram?: string;
  whatsapp?: string;
  companyEmail?: string;
  country?: string;
  city?: string;
  postAddress?: string;
  specialization?: string;
  techStack?: string[];
  grade?: string;
  englishLevel?: string;
  github?: string;
  gitlab?: string;
  linkedin?: string;
  upwork?: string;
  cvLink?: string;
  otherLinks?: string;
  emergency?: EmergencyContact;
  hrNotes?: string;
}

export interface ProfileFinance {
  salary?: number;
  rate?: number;
  currency?: string;
  fop?: boolean;
  payoneerEmail?: string;
  monobankCard?: string;
  privatBankCard?: string;
  taxId?: string;
}

/** What the viewer may see and change of this profile (decided by the server). */
export interface ProfileAccess {
  self: boolean;
  seeContacts: boolean;
  seeGrade: boolean;
  seeHR: boolean;
  seeFinance: boolean;
  editBasics: boolean;
  editWork: boolean;
  editFinance: boolean;
}

export interface Person extends User {
  role: CompanyRole;
  additionRoles: AdditionRole[];
  active: boolean;
  dismissedAt: string | null;
  dismissalReason: string;
  nameCyrillic: string;
  birthday: string | null; // YYYY-MM-DD
  startDate: string | null;
  trialUntil: string | null;
  workingHours: number;
  billable: boolean;
  details: ProfileDetails;
  finance: ProfileFinance | null;
  access: ProfileAccess;
}

export interface PersonFilters {
  q: string;
  quick: "" | "billable" | "nonbillable" | "contractors";
  roles: CompanyRole[];
  specialization: string;
  grade: string;
  englishLevel: string;
  inactive: boolean;
  sort: "name" | "role" | "startDate";
  order: "asc" | "desc";
}

export type DirectoryKind = "specialization" | "tech_stack" | "candidate_channel";

export interface DirectoryItem {
  id: number;
  kind: DirectoryKind;
  name: string;
}

export interface PersonComment {
  id: number;
  userId: number;
  authorId: number | null;
  authorName: string;
  kind: string;
  body: string;
  anonymous: boolean;
  createdAt: string;
}

export interface Feedback {
  id: number;
  userId: number;
  authorId: number | null;
  authorName: string;
  startDoing: string;
  stopDoing: string;
  keepDoing: string;
  anonymous: boolean;
  createdAt: string;
}

export interface PersonDocument {
  id: number;
  userId: number;
  uploaderId: number | null;
  name: string;
  description: string;
  type: string;
  private: boolean;
  link: string;
  fileName: string;
  size: number;
  createdAt: string;
}

export const GRADES = ["Trainee", "Junior", "Strong Junior", "Middle", "Strong Middle", "Senior"];
export const ENGLISH_LEVELS = ["Beginner", "Elementary", "Pre-intermediate", "Intermediate", "Upper-Intermediate", "Advanced", "Proficiency"];
export const EMERGENCY_RELATIONS = ["mother", "father", "wife", "husband", "boyfriend", "girlfriend", "friend", "other"];
export const COMMENT_KINDS = ["candidate_feedback", "english_check", "test_task", "interview_feedback", "rejection_reason", "dismissal", "other"];
export const DOCUMENT_TYPES = ["contract", "nda", "cv", "certificate", "id", "other"];
