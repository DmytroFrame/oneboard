import type { CompanyRole, SpecialType } from "@/types";

export interface Backup {
  id: number;
  size: number;
  tables: number;
  authorName: string; // "" = CRON
  authorRole?: CompanyRole;
  createdAt: string;
}

export interface BackupState {
  backups: Backup[];
  nextAt: string | null;
}

export interface DBSchema {
  database: string;
  migrationVersion: number;
  tables: {
    name: string;
    rows: number;
    columns: { name: string; type: string; nullable: boolean; hasDefault: boolean }[];
    indexes: { name: string; unique: boolean; definition: string }[];
  }[];
  usersByRole: Partial<Record<CompanyRole, number>>;
  hashVersions: Record<string, number>;
}

export interface RuntimeVersion {
  name: string;
  version: string;
}

export interface ProjectFinding {
  projectId: number;
  name: string;
  active: boolean;
  count: number;
  tasks: { id: number; key: string; title: string; specialType: SpecialType }[];
}
