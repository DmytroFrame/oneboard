import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { TimelogModal } from "@/components/TimelogModal/TimelogModal";
import { Button } from "@/components/ui/Button/Button";
import { Muted } from "@/components/ui/Muted/Muted";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { TIMELOGS_CHANGED, timeApi } from "@/lib/api/time";
import { formatDate, formatMinutes } from "@/lib/format";
import type { Card } from "@/types";
import type { Timelog } from "@/types/time";
import s from "./CardWorklog.module.scss";

/** A task's work log, with "log time" right from the task. */
export function CardWorklog({ card }: { card: Card }) {
  const { t, i18n } = useTranslation();
  const { projects } = useLayout();
  const [logs, setLogs] = useState<Timelog[]>([]);
  const [logging, setLogging] = useState(false);

  const load = useCallback(() => {
    timeApi.forCard(card.id).then(setLogs).catch(() => {});
  }, [card.id]);
  useEffect(load, [load]);
  useEffect(() => {
    window.addEventListener(TIMELOGS_CHANGED, load);
    return () => window.removeEventListener(TIMELOGS_CHANGED, load);
  }, [load]);

  return (
    <div className={s.log}>
      <Button type="button" size="sm" onClick={() => setLogging(true)}>
        ⏱ {t("time.logTime")}
      </Button>
      {logs.length === 0 && <Muted as="p">{t("time.workLogEmpty")}</Muted>}
      {logs.map((l) => (
        <div key={l.id} className={s.row}>
          <span className={s.who}>{l.userName}</span>
          <span className={s.when}>{formatDate(l.day, i18n.language)}</span>
          <span className={s.comment}>{l.comment}</span>
          <strong>{formatMinutes(l.minutes)}</strong>
        </div>
      ))}
      {logging && (
        <TimelogModal
          projects={projects.filter((p) => p.id === card.projectId)}
          defaults={{ projectId: card.projectId, card }}
          onSaved={load}
          onClose={() => setLogging(false)}
        />
      )}
    </div>
  );
}
