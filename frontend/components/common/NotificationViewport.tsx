import React, { useEffect } from "react";
import { useNotifications } from "../../context/NotificationContext";

const styleMap = {
  info: {
    border: "border-accent/30",
    badge: "bg-accent/20 text-accent",
  },
  success: {
    border: "border-success/30",
    badge: "bg-success/20 text-success",
  },
  warning: {
    border: "border-warning/30",
    badge: "bg-warning/20 text-warning",
  },
  error: {
    border: "border-danger/30",
    badge: "bg-danger/20 text-danger",
  },
} as const;

export const NotificationViewport: React.FC = () => {
  const { notifications, dismissNotification } = useNotifications();

  useEffect(() => {
    const timers = notifications
      .filter((notification) => notification.durationMs !== 0)
      .map((notification) =>
        window.setTimeout(() => {
          dismissNotification(notification.id);
        }, notification.durationMs ?? 4500),
      );

    return () => {
      for (const timer of timers) {
        window.clearTimeout(timer);
      }
    };
  }, [notifications, dismissNotification]);

  if (notifications.length === 0) return null;

  return (
    <div className="fixed bottom-14 right-4 z-[140] w-[360px] max-w-[calc(100vw-2rem)] space-y-2 pointer-events-none">
      {notifications.map((notification) => {
        const styles = styleMap[notification.kind];
        return (
          <div
            key={notification.id}
            className={`pointer-events-auto bg-card/95 backdrop-blur-sm border ${styles.border} rounded-xl shadow-[0_14px_32px_rgba(0,0,0,0.65)] p-3`}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <span className={`px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-widest ${styles.badge}`}>
                    {notification.kind}
                  </span>
                  <span className="text-[11px] text-helper uppercase tracking-wider">
                    {new Date(notification.createdAt).toLocaleTimeString()}
                  </span>
                </div>
                <p className="text-sm font-semibold text-slate-100 break-words">{notification.title}</p>
                {notification.message && (
                  <p className="text-xs text-slate-400 mt-1 break-words">{notification.message}</p>
                )}
              </div>
              <button
                className="text-helper hover:text-white transition-colors text-xs"
                onClick={() => dismissNotification(notification.id)}
                aria-label="Dismiss notification"
              >
                ✕
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
};
