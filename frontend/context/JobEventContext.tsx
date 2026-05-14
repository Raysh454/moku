import React, { createContext, useContext, useEffect, useRef } from "react";
import { subscribeToJobEvents } from "../src/api/client";
import type { JobEvent } from "../src/api/types";

type JobEventListener = (event: JobEvent) => void;
type JobConnectListener = () => void;

interface JobEventContextType {
  subscribe: (listener: JobEventListener, onConnect?: JobConnectListener) => () => void;
}

const JobEventContext = createContext<JobEventContextType | undefined>(undefined);

export const JobEventProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const listenersRef = useRef<Set<JobEventListener>>(new Set());
  const connectListenersRef = useRef<Set<JobConnectListener>>(new Set());

  useEffect(() => {
    // Global subscription to all job events
    const unsubscribe = subscribeToJobEvents(
      (event) => {
        // Direct notification of all listeners bypasses React state batching
        listenersRef.current.forEach((listener) => listener(event));
      },
      undefined,
      () => {
        // Notification of all connection listeners
        connectListenersRef.current.forEach((listener) => listener());
      },
    );

    return () => {
      unsubscribe();
    };
  }, []);

  const subscribe = (listener: JobEventListener, onConnect?: JobConnectListener) => {
    listenersRef.current.add(listener);
    if (onConnect) connectListenersRef.current.add(onConnect);

    return () => {
      listenersRef.current.delete(listener);
      if (onConnect) connectListenersRef.current.delete(onConnect);
    };
  };

  return (
    <JobEventContext.Provider value={{ subscribe }}>
      {children}
    </JobEventContext.Provider>
  );
};

export const useJobEvents = () => {
  const context = useContext(JobEventContext);
  if (context === undefined) {
    throw new Error("useJobEvents must be used within a JobEventProvider");
  }
  return context;
};

