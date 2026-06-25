import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import {
  applyThemeColors,
  getTheme,
  readStoredThemeId,
  THEME_STORAGE_KEY,
  THEMES,
  type Theme,
  type ThemeId,
} from "../lib/themes";

interface ThemeContextType {
  theme: Theme;
  themeId: ThemeId;
  setThemeId: (id: ThemeId) => void;
  themes: Theme[];
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [themeId, setThemeIdState] = useState<ThemeId>(() => readStoredThemeId());
  const theme = getTheme(themeId);

  useEffect(() => {
    applyThemeColors(theme.colors);
    document.documentElement.dataset.theme = theme.id;
    try {
      localStorage.setItem(THEME_STORAGE_KEY, theme.id);
    } catch {
      // ignore unavailable storage
    }
  }, [theme]);

  const setThemeId = useCallback((id: ThemeId) => setThemeIdState(id), []);

  const value = useMemo<ThemeContextType>(
    () => ({ theme, themeId, setThemeId, themes: THEMES }),
    [theme, themeId, setThemeId],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
};

export const useTheme = (): ThemeContextType => {
  const context = useContext(ThemeContext);
  if (!context) throw new Error("useTheme must be used within a ThemeProvider");
  return context;
};
