import type { ReactNode } from "react";
import { Muted } from "@/components/ui/Muted/Muted";
import s from "./DangerZone.module.scss";

interface Props {
  title: ReactNode;
  text: ReactNode;
  /** The destructive button. */
  action: ReactNode;
}

/** A red box for irreversible actions: delete or leave a project / organization. */
export function DangerZone({ title, text, action }: Props) {
  return (
    <section className={s.zone}>
      <div>
        <strong>{title}</strong>
        <Muted as="p" className={s.text}>
          {text}
        </Muted>
      </div>
      {action}
    </section>
  );
}
