import { StrictMode, lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { Route, Routes } from "react-router-dom";
import "./styles/tokens.css";
import { Providers } from "./app/Providers";
import { consumeFragmentToken } from "./features/account/navigation";
import { GamePage } from "./pages/GamePage";

const emailToken = ["/account/verify", "/account/reset"].includes(
  location.pathname,
)
  ? consumeFragmentToken()
  : null;
const AdminPage = lazy(() => import("./pages/AdminPage"));
const AccountPage = lazy(() => import("./pages/AccountPage"));
const HistoryPage = lazy(() => import("./pages/HistoryPage"));
const ReplayPage = lazy(() => import("./pages/ReplayPage"));
const InvitePage = lazy(() => import("./pages/InvitePage"));
const PvpLobby = lazy(() => import("./pages/PvpLobby"));
const RoomPage = lazy(() => import("./pages/RoomPage"));
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Providers>
      <Suspense fallback={<p>正在载入棋局…</p>}>
        <Routes>
          <Route
            path="/account/*"
            element={<AccountPage emailToken={emailToken} />}
          />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/" element={<GamePage />} />
          <Route path="/games/:id" element={<GamePage />} />
          <Route path="*" element={<GamePage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/replay/:mode/:id" element={<ReplayPage />} />
          <Route path="/pvp" element={<PvpLobby />} />
          <Route path="/join/:code" element={<InvitePage />} />
          <Route path="/rooms/:id" element={<RoomPage />} />
        </Routes>
      </Suspense>
    </Providers>
  </StrictMode>,
);
