import { useTranslation } from "react-i18next";
import type { Priority } from "@/types";
import s from "./PriorityIcon.module.scss";

const ICONS: Record<Priority, { icon: string; color: string }> = {
  low: { icon: "↓", color: "#30a7ff" },
  medium: { icon: "=", color: "#ff9f0a" },
  high: { icon: "↑", color: "#ff6b3d" },
  critical: { icon: "⇈", color: "#ff375f" },
};

/** A coloured arrow for a task's priority. */
export function PriorityIcon({ priority }: { priority: Priority }) {
  const { t } = useTranslation();
  const { icon, color } = ICONS[priority];
  return (
    <span className={s.priority} style={{ color }} title={t("priority.label", { name: t(`priority.${priority}`) })}>
      {icon}
    </span>
  );
}
