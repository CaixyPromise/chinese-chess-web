import { useQuery } from "@tanstack/react-query";
import { Link, useLocation } from "react-router-dom";
import { api } from "../../api/client";
export function AccountNav() {
  const { data } = useQuery({
      queryKey: ["session"],
      queryFn: ({ signal }) => api.session(signal),
    }),
    loc = useLocation();
  const next = encodeURIComponent(loc.pathname + loc.search);
  return data?.identity?.kind === "account" ? (
    <>
      {data.identity.user?.role === "admin" && <Link to="/admin">管理工作台</Link>}
      <Link to="/account">{data.identity.user?.nickname ?? "我的对局"}</Link>
    </>
  ) : (
    <Link to={`/account/login?returnTo=${next}`}>登录 / 注册</Link>
  );
}
