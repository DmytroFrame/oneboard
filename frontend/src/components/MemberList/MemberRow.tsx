import type { ReactNode } from "react";
import s from "./MemberList.module.scss";

interface Props {
  avatar: ReactNode;
  name: ReactNode;
  /** Shown after the name in grey ("(you)"). */
  note?: ReactNode;
  detail: ReactNode;
  /** Role chip, buttons. */
  children?: ReactNode;
}

export function MemberRow({ avatar, name, note, detail, children }: Props) {
  return (
    <div className={s.row}>
      {avatar}
      <div className={s.info}>
        <span>
          {name}
          {note && <em className={s.note}> {note}</em>}
        </span>
        <small>{detail}</small>
      </div>
      {children}
    </div>
  );
}
