import { useLocation } from "react-router-dom";
import { AccountCenter } from "../features/account/AccountCenter";
import AuthPage from "./AuthPage";
export default function AccountPage({
  emailToken,
}: {
  emailToken: string | null;
}) {
  const { pathname } = useLocation();
  return pathname === "/account" || pathname === "/account/" ? (
    <AccountCenter />
  ) : (
    <AuthPage key={pathname} emailToken={emailToken} />
  );
}
