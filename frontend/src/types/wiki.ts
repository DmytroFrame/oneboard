export type WikiLevel = "none" | "read" | "edit";

/** An article in lists and trees (without its text). */
export interface WikiItem {
  id: number;
  parentId: number | null;
  projectId: number | null;
  ownerId: number | null;
  ownerName: string;
  title: string;
  archived: boolean;
  inheritAccess: boolean;
  companyAccess: WikiLevel;
  sharedWith: number;
  level: WikiLevel;
  updatedAt: string;
  updatedByName: string;
}

export interface WikiPage extends WikiItem {
  content: string;
  path: { id: number; title: string }[];
  accessFrom: { id: number; title: string };
}

export interface WikiGrant {
  userId?: number;
  groupId?: number;
  level: Exclude<WikiLevel, "none">;
}

export interface WikiAccess {
  inherit: boolean;
  company: WikiLevel;
  grants: WikiGrant[];
}

export interface WikiVersion {
  id: number;
  articleId: number;
  title: string;
  content?: string;
  authorName: string;
  createdAt: string;
}

export interface WikiGroup {
  id: number;
  name: string;
  ownerId: number | null;
  memberIds: number[];
}

export interface WikiDraft {
  title: string;
  content: string;
  parentId: number | null;
  projectId: number | null;
}

export const COMMENT_CATEGORIES = ["call", "requirements", "design", "estimate", "task_management", "other"] as const;
export type CommentCategory = (typeof COMMENT_CATEGORIES)[number];

export interface ProjectComment {
  id: number;
  projectId: number;
  authorId: number | null;
  authorName: string;
  category: CommentCategory;
  title: string;
  body: string;
  createdAt: string;
  updatedAt: string;
}
