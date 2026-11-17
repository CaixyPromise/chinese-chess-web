import ConfigProvider from "antd/es/config-provider";
import AntApp from "antd/es/app";
import zhCN from "antd/es/locale/zh_CN";
import { BrowserRouter } from "react-router-dom";
import { appTheme } from "./theme";
import { IdentityBoundary } from "./IdentityBoundary";
import type { PropsWithChildren } from "react";
const theme = appTheme();
export function Providers({ children }: PropsWithChildren) {
  return (
    <ConfigProvider locale={zhCN} theme={theme} wave={{ disabled: true }} button={{ autoInsertSpace: false }}>
      <AntApp>
        <IdentityBoundary>
          <BrowserRouter>{children}</BrowserRouter>
        </IdentityBoundary>
      </AntApp>
    </ConfigProvider>
  );
}
