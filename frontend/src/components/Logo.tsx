import { Link } from "react-router-dom";
import { useAuth } from "../auth";

export function Logo() {
  const { user } = useAuth();
  return (
    <Link to={user ? "/home" : "/"} className="logo">
      <svg width="26" height="26" viewBox="0 0 32 32" aria-hidden="true">
        <rect width="32" height="32" rx="7" fill="#0052CC" />
        <rect x="7" y="8" width="5" height="16" rx="1.5" fill="white" />
        <rect x="14" y="8" width="5" height="11" rx="1.5" fill="white" />
        <rect x="21" y="8" width="5" height="7" rx="1.5" fill="white" />
      </svg>
      Flowboard
    </Link>
  );
}
