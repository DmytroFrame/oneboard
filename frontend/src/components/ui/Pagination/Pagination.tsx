import { useTranslation } from "react-i18next";
import { Button } from "../Button/Button";
import s from "./Pagination.module.scss";

interface Props {
  offset: number;
  limit: number;
  total: number;
  onChange: (offset: number) => void;
}

/** "21–40 of 87" with previous / next. */
export function Pagination({ offset, limit, total, onChange }: Props) {
  const { t } = useTranslation();
  if (total <= limit) return null;
  return (
    <div className={s.bar}>
      <span>{t("common.range", { from: offset + 1, to: Math.min(offset + limit, total), total })}</span>
      <Button size="sm" disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - limit))}>
        ←
      </Button>
      <Button size="sm" disabled={offset + limit >= total} onClick={() => onChange(offset + limit)}>
        →
      </Button>
    </div>
  );
}
