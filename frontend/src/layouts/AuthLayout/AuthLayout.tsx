import type { ReactNode } from "react";
import { LanguageSelect } from "@/components/LanguageSelect/LanguageSelect";
import { Logo } from "@/components/Logo/Logo";
import s from "./AuthLayout.module.scss";

/** Sign-in and sign-up: a glass card in the middle of the screen. */
export function AuthLayout({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className={s.page}>
      <div className={s.card}>
        <div className={s.head}>
          <Logo />
          <LanguageSelect />
        </div>
        <h1>{title}</h1>
        {children}
      </div>
    </div>
  );
}

