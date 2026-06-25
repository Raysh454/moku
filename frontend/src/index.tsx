import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import App from "./App";
import { applyThemeColors, getTheme, readStoredThemeId } from "../lib/themes";

// Apply the stored theme before first paint to avoid a flash of the default.
applyThemeColors(getTheme(readStoredThemeId()).colors);

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Could not find root element to mount to");
}

const root = ReactDOM.createRoot(rootElement);
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
