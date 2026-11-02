import { useLocation, useNavigate } from "react-router-dom";

/** After signing in or up: back to the page that asked for it, or home. */
export function useAfterAuth() {
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as { from?: string } | null)?.from ?? "/home";
  return () => navigate(from, { replace: true });
}
