import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Page } from "@/components/Page/Page";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { backwoodsApi, saveBlob } from "@/lib/api/backwoods";
import { can } from "@/lib/companyRoles";
import { formatSize } from "@/lib/format";
import type { CompanyRole } from "@/types";
import type {
  BackupState,
  DBSchema,
  ProjectFinding,
  RuntimeVersion,
} from "@/types/backwoods";
import s from "./Backwoods.module.scss";

/** A check that runs on demand: a button, then its result or an error. */
function useCheck<T>(load: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const run = () => {
    setBusy(true);
    setFailed(false);
    load()
      .then(setData)
      .catch(() => setFailed(true))
      .finally(() => setBusy(false));
  };
  return { data, busy, failed, run };
}

function Section({
  title,
  text,
  actions,
  children,
}: {
  title: string;
  text?: string;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <Panel compact>
      <div className={s.head}>
        <h2>{title}</h2>
        <div className={s.actions}>{actions}</div>
      </div>
      {text && (
        <Muted as="p" className={s.text}>
          {text}
        </Muted>
      )}
      {children}
    </Panel>
  );
}

/** The admin panel: backups, the database schema, versions and data checks. */
export default function Backwoods() {
  const { t, i18n } = useTranslation();
  const [backups, setBackups] = useState<BackupState | null>(null);
  const [backingUp, setBackingUp] = useState(false);
  const [backupError, setBackupError] = useState("");
  const schema = useCheck<DBSchema>(backwoodsApi.schema);
  const versions = useCheck<RuntimeVersion[]>(backwoodsApi.versions);
  const special = useCheck<ProjectFinding[]>(backwoodsApi.specialTasks);
  const comments = useCheck<ProjectFinding[]>(backwoodsApi.commentedProjects);
  const [openTasks, setOpenTasks] = useState<number | null>(null);
  const { user } = useAuth();
  // The installation's tools are for platform admins; organization owners and admins check their data.
  const platform = !!user?.platformAdmin;
  const orgChecks = can(user, "openBackwoods");

  useEffect(() => {
    if (!platform) return;
    backwoodsApi
      .backups()
      .then(setBackups)
      .catch((err) => setBackupError(err.message));
    versions.run();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const when = (d: string) =>
    new Intl.DateTimeFormat(i18n.language, {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(d));
  const makeBackup = () => {
    setBackingUp(true);
    setBackupError("");
    backwoodsApi
      .backup()
      .then(setBackups)
      .catch((err) =>
        setBackupError(err.message || t("backwoods.backupFailed")),
      )
      .finally(() => setBackingUp(false));
  };
  const runLabel = (c: { data: unknown }) =>
    c.data ? t("backwoods.runAgain") : t("backwoods.run");

  return (
    <Page width="medium">
      <h1 className={s.title}>{t("backwoods.title")}</h1>
      {orgChecks && user?.organization && (
        <Muted as="p" className={s.text}>
          {t("backwoods.orgScope", { name: user.organization.name })}
        </Muted>
      )}

      {platform && (
        <>
          <Section
            title={t("backwoods.backups")}
            text={t("backwoods.backupsText")}
            actions={
              <Button
                variant="primary"
                size="sm"
                disabled={backingUp || !!backups?.nextAt}
                onClick={makeBackup}
              >
                {backingUp
                  ? t("backwoods.backingUp")
                  : t("backwoods.makeBackup")}
              </Button>
            }
          >
            {backups?.nextAt && (
              <Muted as="p" className={s.text}>
                {t("backwoods.nextBackup", { date: when(backups.nextAt) })}
              </Muted>
            )}
            {backupError && <Banner>{backupError}</Banner>}
            {backups && backups.backups.length === 0 && (
              <Muted as="p" className={s.text}>
                {t("backwoods.noBackups")}
              </Muted>
            )}
            {backups && backups.backups.length > 0 && (
              <>
                <h3 className={s.subhead}>{t("backwoods.lastBackups")}</h3>
                <ul className={s.list}>
                  {backups.backups.map((b) => (
                    <li key={b.id}>
                      <span>
                        {when(b.createdAt)} ·{" "}
                        {b.authorName
                          ? `${b.authorName}${b.authorRole ? ` · ${t(`companyRoles.${b.authorRole}`)}` : ""}`
                          : "CRON"}
                      </span>
                      <Muted>
                        {formatSize(b.size, i18n.language)} ·{" "}
                        {t("backwoods.tables", { count: b.tables })}
                      </Muted>
                      <Button
                        size="sm"
                        onClick={() =>
                          backwoodsApi
                            .download(
                              b.id,
                              `oneboard-backup-${b.createdAt.slice(0, 19)}.tar.gz`,
                            )
                            .catch(() =>
                              setBackupError(t("backwoods.downloadFailed")),
                            )
                        }
                      >
                        {t("backwoods.download")}
                      </Button>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </Section>

          <Section
            title={t("backwoods.schema")}
            text={t("backwoods.schemaText")}
            actions={
              <>
                <Button size="sm" disabled={schema.busy} onClick={schema.run}>
                  {runLabel(schema)}
                </Button>
                {schema.data && (
                  <Button
                    size="sm"
                    onClick={() =>
                      saveBlob(
                        new Blob([JSON.stringify(schema.data, null, 2)], {
                          type: "application/json",
                        }),
                        `oneboard-schema.json`,
                      )
                    }
                  >
                    {t("backwoods.downloadJson")}
                  </Button>
                )}
              </>
            }
          >
            {schema.failed && <Banner>{t("backwoods.checkFailed")}</Banner>}
            {schema.data && (
              <div className={s.schema}>
                <p>
                  {t("backwoods.database", { name: schema.data.database })} ·{" "}
                  {t("backwoods.migration", {
                    version: schema.data.migrationVersion,
                  })}{" "}
                  ·{" "}
                  {t("backwoods.tables", { count: schema.data.tables.length })}
                </p>
                <p>
                  {t("backwoods.usersByRole")}{" "}
                  {(
                    Object.entries(schema.data.usersByRole) as [
                      CompanyRole,
                      number,
                    ][]
                  )
                    .map(([role, n]) => `${t(`companyRoles.${role}`)}: ${n}`)
                    .join(", ")}
                </p>
                <p>
                  {t("backwoods.hashVersions")}{" "}
                  {Object.entries(schema.data.hashVersions)
                    .map(([v, n]) => `${v}: ${n}`)
                    .join(", ")}
                </p>
                {schema.data.tables.map((tb) => (
                  <details key={tb.name}>
                    <summary>
                      <code>{tb.name}</code>{" "}
                      <Muted>{t("backwoods.rows", { count: tb.rows })}</Muted>
                    </summary>
                    <ul className={s.columns}>
                      {tb.columns.map((c) => (
                        <li key={c.name}>
                          <code>{c.name}</code>{" "}
                          <Muted>
                            {c.type}
                            {c.nullable ? "?" : ""}
                            {c.hasDefault ? " = …" : ""}
                          </Muted>
                        </li>
                      ))}
                    </ul>
                    <ul className={s.columns}>
                      {tb.indexes.map((i) => (
                        <li key={i.name}>
                          <code>{i.name}</code>{" "}
                          {i.unique && <Chip>{t("backwoods.unique")}</Chip>}
                        </li>
                      ))}
                    </ul>
                  </details>
                ))}
              </div>
            )}
          </Section>

          <Section
            title={t("backwoods.versions")}
            text={t("backwoods.versionsText")}
            actions={
              <Button size="sm" disabled={versions.busy} onClick={versions.run}>
                {runLabel(versions)}
              </Button>
            }
          >
            {versions.failed && <Banner>{t("backwoods.checkFailed")}</Banner>}
            {versions.data && (
              <dl className={s.versions}>
                {versions.data.map((v) => (
                  <div key={v.name}>
                    <dt>{v.name}</dt>
                    <dd>{v.version || t("backwoods.unknown")}</dd>
                  </div>
                ))}
              </dl>
            )}
          </Section>
        </>
      )}

      {orgChecks && (
        <>
          <Section
            title={t("backwoods.specialTasks")}
            text={t("backwoods.specialTasksText")}
            actions={
              <Button size="sm" disabled={special.busy} onClick={special.run}>
                {runLabel(special)}
              </Button>
            }
          >
            {special.failed && <Banner>{t("backwoods.checkFailed")}</Banner>}
            {special.data && special.data.length === 0 && (
              <Muted as="p" className={s.text}>
                {t("backwoods.noSpecialTasks")}
              </Muted>
            )}
            {special.data && special.data.length > 0 && (
              <>
                <p className={s.text}>
                  {t("backwoods.specialFound", { count: special.data.length })}
                </p>
                <ul className={s.list}>
                  {special.data.map((f) => (
                    <li key={f.projectId} className={s.finding}>
                      <span>
                        <Link to={`/projects/${f.projectId}`}>{f.name}</Link>{" "}
                        {!f.active && <Chip>{t("backwoods.inactive")}</Chip>}
                      </span>
                      <Button
                        size="sm"
                        onClick={() =>
                          setOpenTasks(
                            openTasks === f.projectId ? null : f.projectId,
                          )
                        }
                      >
                        {t("backwoods.showTasks", { count: f.count })}
                      </Button>
                      {openTasks === f.projectId && (
                        <ul className={s.tasks}>
                          {f.tasks.map((task) => (
                            <li key={task.id}>
                              <Link
                                to={`/projects/${f.projectId}?card=${task.id}`}
                              >
                                {task.key}
                              </Link>{" "}
                              {task.title}{" "}
                              <Chip>
                                {t(`tasks.specialTypes.${task.specialType}`)}
                              </Chip>
                            </li>
                          ))}
                        </ul>
                      )}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </Section>

          <Section
            title={t("backwoods.commentedProjects")}
            text={t("backwoods.commentedText")}
            actions={
              <Button size="sm" disabled={comments.busy} onClick={comments.run}>
                {runLabel(comments)}
              </Button>
            }
          >
            {comments.failed && <Banner>{t("backwoods.checkFailed")}</Banner>}
            {comments.data && comments.data.length === 0 && (
              <Muted as="p" className={s.text}>
                {t("backwoods.noComments")}
              </Muted>
            )}
            {comments.data && comments.data.length > 0 && (
              <ul className={s.list}>
                {comments.data.map((f) => (
                  <li key={f.projectId}>
                    <span>
                      <a
                        href={`/projects/${f.projectId}/comments`}
                        target="_blank"
                        rel="noreferrer"
                      >
                        {f.name}
                      </a>{" "}
                      {!f.active && <Chip>{t("backwoods.inactive")}</Chip>}
                    </span>
                    <Muted>{t("backwoods.comments", { count: f.count })}</Muted>
                  </li>
                ))}
              </ul>
            )}
          </Section>
        </>
      )}
    </Page>
  );
}
