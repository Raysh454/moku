import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

export type NotificationKind = "info" | "success" | "warning" | "error";

export type AppNotification = {
  id: string;
  kind: NotificationKind;
  title: string;
  message?: string;
  durationMs?: number;
  createdAt: number;
};

type NotificationInput = {
  kind?: NotificationKind;
  title: string;
  message?: string;
  durationMs?: number;
};

type NotificationContextType = {
  notifications: AppNotification[];
  notify: (input: NotificationInput) => string;
  dismissNotification: (id: string) => void;
  clearNotifications: () => void;
};

const NotificationContext = createContext<NotificationContextType | undefined>(undefined);

function buildId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export const NotificationProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [notifications, setNotifications] = useState<AppNotification[]>([]);

  const dismissNotification = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const clearNotifications = useCallback(() => {
    setNotifications([]);
  }, []);

  const notify = useCallback((input: NotificationInput): string => {
    const id = buildId();
    const entry: AppNotification = {
      id,
      kind: input.kind || "info",
      title: input.title,
      message: input.message,
      durationMs: input.durationMs ?? 4500,
      createdAt: Date.now(),
    };

    setNotifications((prev) => [...prev, entry]);
    return id;
  }, []);

  const value = useMemo<NotificationContextType>(
    () => ({ notifications, notify, dismissNotification, clearNotifications }),
    [notifications, notify, dismissNotification, clearNotifications],
  );

  return <NotificationContext.Provider value={value}>{children}</NotificationContext.Provider>;
};

export const useNotifications = () => {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error("useNotifications must be used within a NotificationProvider");
  }
  return context;
};
