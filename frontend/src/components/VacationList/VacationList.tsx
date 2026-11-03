import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { Muted } from "@/components/ui/Muted/Muted";
import { Table } from "@/components/ui/Table/Table";
import { formatDate } from "@/lib/format";
import type { Vacation } from "@/types/time";

interface Props {
  items: Vacation[];
  showRequester?: boolean;
  onDecide?: (v: Vacation, status: "approved" | "declined") => void;
  onEdit?: (v: Vacation) => void;
  onDelete?: (v: Vacation) => void;
}

const TONE = { pending: "warning", approved: "success", declined: "danger" } as const;

/** Absence requests in a table, with the actions the page allows. */
export function VacationList({ items, showRequester, onDecide, onEdit, onDelete }: Props) {
  const { t, i18n } = useTranslation();
  if (!items.length) return <Muted as="p">{t("vacations.empty")}</Muted>;
  const date = (d: string) => formatDate(d, i18n.language);
  return (
    <Table>
      <thead>
        <tr>
          {showRequester && <th>{t("vacations.requester")}</th>}
          <th>{t("vacations.type")}</th>
          <th>{t("vacations.period")}</th>
          <th>{t("vacations.comment")}</th>
          <th />
          <th />
        </tr>
      </thead>
      <tbody>
        {items.map((v) => (
          <tr key={v.id}>
            {showRequester && <td>{v.userName}</td>}
            <td>{t(`vacations.types.${v.type}`)}</td>
            <td>
              {date(v.start)} – {date(v.end)} <Muted>({t("vacations.days", { count: v.days })})</Muted>
            </td>
            <td style={{ whiteSpace: "normal" }}>{v.comment}</td>
            <td>
              <Chip tone={TONE[v.status]}>{t(`vacations.statuses.${v.status}`)}</Chip>
            </td>
            <td style={{ textAlign: "right" }}>
              {onDecide && v.status === "pending" && (
                <>
                  <Button size="sm" variant="primary" onClick={() => onDecide(v, "approved")}>
                    {t("vacations.approve")}
                  </Button>{" "}
                  <Button size="sm" variant="danger" onClick={() => onDecide(v, "declined")}>
                    {t("vacations.decline")}
                  </Button>
                </>
              )}
              {onEdit && v.status === "pending" && (
                <Button size="sm" variant="link" onClick={() => onEdit(v)}>
                  ✎
                </Button>
              )}
              {onDelete && (
                <Button size="sm" variant="danger" onClick={() => onDelete(v)}>
                  {t("vacations.delete")}
                </Button>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </Table>
  );
}
