import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { Avatar } from "@/components/Avatar/Avatar";
import { Logo } from "@/components/Logo/Logo";
import { Button } from "@/components/ui/Button/Button";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { MenuIcon } from "@/components/ui/icons";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { useAuth } from "@/context/auth";
import s from "./Topbar.module.scss";

interface Props {
  drawerOpen: boolean;
  onToggleDrawer: () => void;
}

/** The app's top bar: the drawer button (phones), logo, the signed-in user. */
export function Topbar({ drawerOpen, onToggleDrawer }: Props) {
  const { t } = useTranslation();
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  return (
    <header className={s.topbar}>
      <IconButton className={s.menuToggle} aria-label={t("nav.menu")} aria-expanded={drawerOpen} onClick={onToggleDrawer}>
        <MenuIcon />
      </IconButton>
      <Logo />
      <Spacer />
      <div className={s.user}>
        <Avatar user={user} size={30} onClick={() => navigate("/settings")} />
        <span className={s.name}>{user!.name}</span>
        <Button size="sm" onClick={logout}>
          {t("layout.logout")}
        </Button>
      </div>
    </header>
  );
}
