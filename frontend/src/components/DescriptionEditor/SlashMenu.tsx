import type { SuggestionProps } from "@tiptap/suggestion";
import { forwardRef, useEffect, useImperativeHandle, useState } from "react";
import { useTranslation } from "react-i18next";
import { cx } from "@/lib/cx";
import type { SlashItem } from "./slashCommand";
import s from "./SlashMenu.module.scss";

// data-slash-menu: the card modal leaves Escape to this menu while it is open.

export interface MenuHandle {
  onKeyDown: (e: KeyboardEvent) => boolean;
}

export const SlashMenu = forwardRef<MenuHandle, SuggestionProps<SlashItem>>(function SlashMenu({ items, command }, ref) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  useEffect(() => setIndex(0), [items]);

  useImperativeHandle(ref, () => ({
    onKeyDown: (e) => {
      if (!items.length) return false;
      if (e.key === "ArrowDown") setIndex((i) => (i + 1) % items.length);
      else if (e.key === "ArrowUp") setIndex((i) => (i + items.length - 1) % items.length);
      else if (e.key === "Enter") command(items[index]!);
      else return false;
      return true;
    },
  }));

  if (!items.length) return <div className={cx(s.menu, s.empty)} data-slash-menu>{t("editor.noMatches")}</div>;
  return (
    <div className={s.menu} role="listbox" data-slash-menu>
      {items.map((item, i) => (
        <button
          key={item.id}
          type="button"
          role="option"
          aria-selected={i === index}
          className={cx(s.item, i === index && s.itemActive)}
          onMouseEnter={() => setIndex(i)}
          onMouseDown={(e) => (e.preventDefault(), command(item))}
        >
          <span className={s.icon}>{item.icon}</span>
          <span className={s.text}>
            <span>{t(`editor.blocks.${item.id}`)}</span>
            <small>{t(`editor.hints.${item.id}`)}</small>
          </span>
        </button>
      ))}
    </div>
  );
});
