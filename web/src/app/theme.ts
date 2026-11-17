import type { ThemeConfig } from "antd/es/config-provider/context";

export function appTheme(): ThemeConfig {
  const style = getComputedStyle(document.documentElement);
  const token = (name: string) => style.getPropertyValue(name).trim();
  return {
    components: { Button: { primaryShadow: "none", defaultShadow: "none", dangerShadow: "none" } },
    token: {
      colorPrimary: token("--accent"),
      colorText: token("--ink"),
      colorTextSecondary: token("--muted"),
      colorBorder: token("--line"),
      colorBgContainer: token("--surface"),
      colorBgElevated: token("--surface"),
      colorBgLayout: token("--paper"),
      colorLink: token("--ink"),
      colorLinkHover: token("--accent"),
      borderRadius: Number.parseFloat(token("--radius")),
      fontFamily: token("--font-body"),
      controlHeight: 40,
      motion: !matchMedia("(prefers-reduced-motion: reduce)").matches,
    },
  };
}
