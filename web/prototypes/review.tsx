import { createRoot } from "react-dom/client";
import "../src/styles/tokens.css";
import { Providers } from "../src/app/Providers";
import { ReviewPrototype } from "./ReviewPrototype";

createRoot(document.getElementById("root")!).render(
  <Providers>
    <ReviewPrototype />
  </Providers>,
);
