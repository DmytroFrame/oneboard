import type { ReactNode } from "react";
import s from "./EmptyState.module.scss";

interface Props {
  icon: ReactNode;
  title: ReactNode;
  text: ReactNode;
  action?: ReactNode;
}

export function EmptyState({ icon, title, text, action }: Props) {
  return (
    <div className={s.empty}>
      <div className={s.icon}>{icon}</div>
      <h2>{title}</h2>
      <p>{text}</p>
      {action}
    </div>
  );
}
